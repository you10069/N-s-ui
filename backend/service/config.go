package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"s-ui/config"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"s-ui/singbox"
	"s-ui/util"
	"strconv"
	"time"
)

var ApiAddr string
var LastUpdate int64
var IsSystemd bool

type ConfigService struct {
	ClientService
	TlsService
	InDataService
	singbox.Controller
	SettingService
}

type SingBoxConfig struct {
	Log          json.RawMessage   `json:"log"`
	Dns          json.RawMessage   `json:"dns"`
	Ntp          json.RawMessage   `json:"ntp"`
	Inbounds     []json.RawMessage `json:"inbounds"`
	Outbounds    []json.RawMessage `json:"outbounds"`
	Endpoints    []json.RawMessage `json:"endpoints,omitempty"`
	Route        json.RawMessage   `json:"route"`
	Experimental json.RawMessage   `json:"experimental"`
}

func NewConfigService() *ConfigService {
	return &ConfigService{}
}

func (s *ConfigService) InitConfig() error {
	IsSystemd = config.IsSystemd()
	configPath := config.GetBinFolderPath()
	data, err := os.ReadFile(configPath + "/config.json")
	if err != nil {
		if os.IsNotExist(err) {
			defaultConfig := []byte(config.GetDefaultConfig())
			err = os.MkdirAll(configPath, 01764)
			if err != nil {
				return err
			}
			err = os.WriteFile(configPath+"/config.json", defaultConfig, 0764)
			if err != nil {
				return err
			}
			data = defaultConfig
		} else {
			return err
		}
	}
	singboxConfig, err := decodeSingBoxConfig(data)
	if err != nil {
		return err
	}
	if err = atomicWriteFile(s.Controller.GetActiveConfigPath(), data, 0644); err != nil {
		return err
	}
	if err = s.RefreshApiAddr(singboxConfig); err != nil {
		return err
	}
	_, err = NewRuntimeConfigService().Reconcile(true, true)
	return err
}

func (s *ConfigService) GetConfig() (*SingBoxConfig, error) {
	configPath := config.GetBinFolderPath()
	data, err := os.ReadFile(configPath + "/config.json")
	if err != nil {
		return nil, err
	}
	return decodeSingBoxConfig(data)
}

func decodeSingBoxConfig(data []byte) (*SingBoxConfig, error) {
	clean, err := util.StripJSONComments(data)
	if err != nil {
		return nil, err
	}
	var singboxConfig SingBoxConfig
	decoder := json.NewDecoder(bytes.NewReader(clean))
	if err = decoder.Decode(&singboxConfig); err != nil {
		return nil, err
	}
	return &singboxConfig, nil
}

// GetRawConfig returns the exact config text, including comments and formatting.
func (s *ConfigService) GetRawConfig() (string, int64, error) {
	path := s.Controller.GetConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	return string(data), info.ModTime().Unix(), nil
}

// GetRuntimeConfig returns the exact generated runtime configuration currently
// used by sing-box. The runtime file is read-only from the UI because it is
// rebuilt automatically from config.active.json and client limit state.
func (s *ConfigService) GetRuntimeConfig() (string, int64, bool, error) {
	path := s.Controller.GetRuntimeConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, false, err
	}
	return string(data), info.ModTime().Unix(), true, nil
}

func (s *ConfigService) IsReloadPending() (bool, error) {
	source, err := os.ReadFile(s.Controller.GetConfigPath())
	if err != nil {
		return false, err
	}
	active, err := os.ReadFile(s.Controller.GetActiveConfigPath())
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !bytes.Equal(source, active), nil
}

// CheckRawConfig validates both JSONC syntax and the complete configuration
// with the actual sing-box binary installed next to s-ui.
func (s *ConfigService) CheckRawConfig(content string) error {
	if _, err := decodeSingBoxConfig([]byte(content)); err != nil {
		return fmt.Errorf("invalid JSON/JSONC: %w", err)
	}

	configDir := config.GetBinFolderPath()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	tempFile, err := os.CreateTemp(configDir, ".config-check-*.json")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err = tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return err
	}
	if err = tempFile.Close(); err != nil {
		return err
	}

	cmd := exec.Command(s.Controller.GetBinaryPath(), "check", "-c", tempPath)
	cmd.Dir = configDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := string(bytes.TrimSpace(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("sing-box check failed: %s", message)
	}
	return nil
}

// ReloadCore validates the source configuration, rebuilds the filtered runtime
// configuration and then fully restarts sing-box so existing blocked-user
// connections are closed immediately.
func (s *ConfigService) ReloadCore() error {
	content, _, err := s.GetRawConfig()
	if err != nil {
		return err
	}
	if err = s.CheckRawConfig(content); err != nil {
		return err
	}
	parsed, err := decodeSingBoxConfig([]byte(content))
	if err != nil {
		return err
	}
	if err = atomicWriteFile(s.Controller.GetActiveConfigPath(), []byte(content), 0644); err != nil {
		return err
	}
	if err = s.RefreshApiAddr(parsed); err != nil {
		return err
	}
	if _, err = NewRuntimeConfigService().Reconcile(true, false); err != nil {
		return err
	}
	return s.Controller.Restart()
}

// SaveRawConfig writes the exact user text and intentionally does not restart
// sing-box. Reload is a separate explicit action in the UI.
func (s *ConfigService) SaveRawConfig(content string) error {
	if err := s.CheckRawConfig(content); err != nil {
		return err
	}

	configPath := s.Controller.GetConfigPath()
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	mode := os.FileMode(0644)
	if info, err := os.Stat(configPath); err == nil {
		mode = info.Mode().Perm()
	}
	tempFile, err := os.CreateTemp(configDir, ".config-save-*.json")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if err = tempFile.Chmod(mode); err != nil {
		tempFile.Close()
		return err
	}
	if _, err = tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return err
	}
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return err
	}
	if err = tempFile.Close(); err != nil {
		return err
	}

	// Keep one last-known file for quick recovery from operational mistakes.
	if current, readErr := os.ReadFile(configPath); readErr == nil {
		_ = os.WriteFile(configPath+".bak", current, mode)
	}
	if err = os.Rename(tempPath, configPath); err != nil {
		return err
	}

	LastUpdate = time.Now().Unix()
	return nil
}

func (s *ConfigService) SaveChanges(changes map[string]string, loginUser string) error {
	var err error
	var clientChanges, tlsChanges, inChanges, settingChanges, configChanges []model.Changes
	if _, ok := changes["clients"]; ok {
		err = json.Unmarshal([]byte(changes["clients"]), &clientChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["tls"]; ok {
		err = json.Unmarshal([]byte(changes["tls"]), &tlsChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["inData"]; ok {
		err = json.Unmarshal([]byte(changes["inData"]), &inChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["settings"]; ok {
		err = json.Unmarshal([]byte(changes["settings"]), &settingChanges)
		if err != nil {
			return err
		}
	}
	if _, ok := changes["config"]; ok {
		err = json.Unmarshal([]byte(changes["config"]), &configChanges)
		if err != nil {
			return err
		}
	}

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	if len(clientChanges) > 0 {
		err = s.ClientService.Save(tx, clientChanges)
		if err != nil {
			return err
		}
	}
	if len(tlsChanges) > 0 {
		err = s.TlsService.Save(tx, tlsChanges)
		if err != nil {
			return err
		}
	}
	if len(inChanges) > 0 {
		err = s.InDataService.Save(tx, inChanges)
		if err != nil {
			return err
		}
	}
	if len(settingChanges) > 0 {
		err = s.SettingService.Save(tx, settingChanges)
		if err != nil {
			return err
		}
	}
	if len(configChanges) > 0 {
		singboxConfig, err := s.GetConfig()
		if err != nil {
			return err
		}
		newConfig := *singboxConfig
		for _, change := range configChanges {
			rawObject := change.Obj
			switch change.Key {
			case "all":
				err = json.Unmarshal(rawObject, &newConfig)
				if err != nil {
					return err
				}
			case "log":
				newConfig.Log = rawObject
			case "dns":
				newConfig.Dns = rawObject
			case "ntp":
				newConfig.Ntp = rawObject
			case "route":
				newConfig.Route = rawObject
			case "experimental":
				newConfig.Experimental = rawObject
			case "inbounds":
				if change.Action == "edit" {
					newConfig.Inbounds[change.Index] = rawObject
				} else if change.Action == "del" {
					newConfig.Inbounds = append(newConfig.Inbounds[:change.Index], newConfig.Inbounds[change.Index+1:]...)
				} else {
					newConfig.Inbounds = append(newConfig.Inbounds, rawObject)
				}
			case "outbounds":
				if change.Action == "edit" {
					newConfig.Outbounds[change.Index] = rawObject
				} else if change.Action == "del" {
					newConfig.Outbounds = append(newConfig.Outbounds[:change.Index], newConfig.Outbounds[change.Index+1:]...)
				} else {
					newConfig.Outbounds = append(newConfig.Outbounds, rawObject)
				}
			}
		}

		err = s.Save(&newConfig)
		if err != nil {
			return err
		}
	}

	// Log changes
	dt := time.Now().Unix()
	allChanges := append(clientChanges, settingChanges...)
	allChanges = append(allChanges, configChanges...)
	allChanges = append(allChanges, tlsChanges...)
	allChanges = append(allChanges, inChanges...)
	if len(allChanges) > 0 {
		for index := range allChanges {
			allChanges[index].DateTime = dt
			allChanges[index].Actor = loginUser
		}
		err = tx.Model(model.Changes{}).Create(&allChanges).Error
		if err != nil {
			return err
		}
	}

	LastUpdate = dt

	return nil
}

func (s *ConfigService) CheckChanges(lu string) (bool, error) {
	if lu == "" {
		return true, nil
	}
	if LastUpdate == 0 {
		db := database.GetDB()
		var count int64
		err := db.Model(model.Changes{}).Where("date_time > " + lu).Count(&count).Error
		if err == nil {
			LastUpdate = time.Now().Unix()
		}
		return count > 0, err
	} else {
		intLu, err := strconv.ParseInt(lu, 10, 64)
		return LastUpdate > intLu, err
	}
}

func (s *ConfigService) Save(singboxConfig *SingBoxConfig) error {
	data, err := json.MarshalIndent(singboxConfig, "", "  ")
	if err != nil {
		return err
	}
	if err = s.SaveRawConfig(string(data)); err != nil {
		return err
	}
	return s.ReloadCore()
}

func (s *ConfigService) RefreshApiAddr(singboxConfig *SingBoxConfig) error {
	Env_API := config.GetEnvApi()
	if len(Env_API) > 0 {
		ApiAddr = Env_API
	} else {
		var err error
		if singboxConfig == nil {
			singboxConfig, err = s.GetConfig()
			if err != nil {
				return err
			}

		}

		var experimental struct {
			V2rayApi struct {
				Listen string      `json:"listen"`
				Stats  interface{} `jaon:"stats"`
			} `json:"v2ray_api"`
		}
		err = json.Unmarshal(singboxConfig.Experimental, &experimental)
		if err != nil {
			return err
		}

		ApiAddr = experimental.V2rayApi.Listen
	}
	return nil
}

func (s *ConfigService) GetChanges(actor string, chngKey string, count string) []model.Changes {
	c, _ := strconv.Atoi(count)
	whereString := "`id`>0"
	if len(actor) > 0 {
		whereString += " and `actor`='" + actor + "'"
	}
	if len(chngKey) > 0 {
		whereString += " and `key`='" + chngKey + "'"
	}
	db := database.GetDB()
	var chngs []model.Changes
	err := db.Model(model.Changes{}).Where(whereString).Order("`id` desc").Limit(c).Scan(&chngs).Error
	if err != nil {
		logger.Warning(err)
	}
	return chngs
}
