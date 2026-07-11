package service

import (
	"encoding/json"
	"os"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"s-ui/util/common"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var defaultValueMap = map[string]string{
	"webListen":     "",
	"webDomain":     "",
	"webPort":       "2095",
	"secret":        common.Random(32),
	"webCertFile":   "",
	"webKeyFile":    "",
	"webPath":       "/app/",
	"webURI":        "",
	"sessionMaxAge": "0",
	"trafficAge":    "30",
	"timeLocation":  "Asia/Tehran",
}

// These keys belonged to the removed subscription and JSON-subscription
// servers. They are deleted from existing databases during the next settings
// read so upgrades do not retain dead configuration.
var obsoleteSettingKeys = []string{
	"subListen",
	"subPort",
	"subPath",
	"subDomain",
	"subCertFile",
	"subKeyFile",
	"subUpdates",
	"subEncode",
	"subShowInfo",
	"subURI",
	"subJsonExt",
}

type SettingService struct {
}

func (s *SettingService) GetAllSetting() (*map[string]string, error) {
	db := database.GetDB()
	if err := db.Where("key IN ?", obsoleteSettingKeys).Delete(&model.Setting{}).Error; err != nil {
		return nil, err
	}

	settings := make([]*model.Setting, 0)
	if err := db.Model(model.Setting{}).Find(&settings).Error; err != nil {
		return nil, err
	}
	allSetting := map[string]string{}

	for _, setting := range settings {
		allSetting[setting.Key] = setting.Value
	}

	for key, defaultValue := range defaultValueMap {
		if _, exists := allSetting[key]; !exists {
			if err := s.saveSetting(key, defaultValue); err != nil {
				return nil, err
			}
			allSetting[key] = defaultValue
		}
	}

	// Never expose the session signing secret through the web API.
	delete(allSetting, "secret")

	return &allSetting, nil
}

func (s *SettingService) ResetSettings() error {
	db := database.GetDB()
	return db.Where("1 = 1").Delete(model.Setting{}).Error
}

func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *SettingService) getString(key string) (string, error) {
	setting, err := s.getSetting(key)
	if database.IsNotFound(err) {
		value, ok := defaultValueMap[key]
		if !ok {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return value, nil
	} else if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (s *SettingService) saveSetting(key string, value string) error {
	setting, err := s.getSetting(key)
	db := database.GetDB()
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{
			Key:   key,
			Value: value,
		}).Error
	} else if err != nil {
		return err
	}
	setting.Key = key
	setting.Value = value
	return db.Save(setting).Error
}

func (s *SettingService) setString(key string, value string) error {
	return s.saveSetting(key, value)
}

func (s *SettingService) getInt(key string) (int, error) {
	str, err := s.getString(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(str)
}

func (s *SettingService) setInt(key string, value int) error {
	return s.setString(key, strconv.Itoa(value))
}

func (s *SettingService) GetListen() (string, error) {
	return s.getString("webListen")
}

func (s *SettingService) GetWebDomain() (string, error) {
	return s.getString("webDomain")
}

func (s *SettingService) GetPort() (int, error) {
	return s.getInt("webPort")
}

func (s *SettingService) SetPort(port int) error {
	return s.setInt("webPort", port)
}

func (s *SettingService) GetCertFile() (string, error) {
	return s.getString("webCertFile")
}

func (s *SettingService) GetKeyFile() (string, error) {
	return s.getString("webKeyFile")
}

func (s *SettingService) GetWebPath() (string, error) {
	webPath, err := s.getString("webPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return webPath, nil
}

func (s *SettingService) SetWebPath(webPath string) error {
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return s.setString("webPath", webPath)
}

func (s *SettingService) GetSecret() ([]byte, error) {
	secret, err := s.getString("secret")
	if secret == defaultValueMap["secret"] {
		if saveErr := s.saveSetting("secret", secret); saveErr != nil {
			logger.Warning("save secret failed:", saveErr)
		}
	}
	return []byte(secret), err
}

func (s *SettingService) GetSessionMaxAge() (int, error) {
	return s.getInt("sessionMaxAge")
}

func (s *SettingService) GetTrafficAge() (int, error) {
	return s.getInt("trafficAge")
}

func (s *SettingService) GetTimeLocation() (*time.Location, error) {
	locationName, err := s.getString("timeLocation")
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(locationName)
	if err != nil {
		defaultLocation := defaultValueMap["timeLocation"]
		logger.Errorf("location <%v> not exist, using default location: %v", locationName, defaultLocation)
		return time.LoadLocation(defaultLocation)
	}
	return location, nil
}

func (s *SettingService) Save(tx *gorm.DB, changes []model.Changes) error {
	for _, change := range changes {
		key := change.Key
		if _, allowed := defaultValueMap[key]; !allowed || key == "secret" {
			continue
		}

		var value string
		if err := json.Unmarshal(change.Obj, &value); err != nil {
			return err
		}

		if value != "" && (key == "webCertFile" || key == "webKeyFile") {
			if err := s.fileExists(value); err != nil {
				return common.NewError(" -> ", value, " is not exists")
			}
		}

		if key == "webPath" {
			if !strings.HasPrefix(value, "/") {
				value = "/" + value
			}
			if !strings.HasSuffix(value, "/") {
				value += "/"
			}
		}

		if err := tx.Model(model.Setting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *SettingService) fileExists(path string) error {
	_, err := os.Stat(path)
	return err
}
