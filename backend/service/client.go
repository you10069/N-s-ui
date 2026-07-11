package service

import (
	"encoding/json"
	"fmt"
	"os"
	"s-ui/config"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"s-ui/util"
	"sort"
	"time"

	"gorm.io/gorm"
)

type ClientService struct {
}

func (s *ClientService) GetAll() ([]model.Client, error) {
	db := database.GetDB()
	clients := []model.Client{}
	err := db.Model(model.Client{}).Where("in_config = ?", true).Order("name asc").Scan(&clients).Error
	if err != nil {
		return nil, err
	}
	for index := range clients {
		if clients[index].Multiplier <= 0 {
			clients[index].Multiplier = 1
		}
		clients[index].Adjusted = adjustedTraffic(clients[index].Up, clients[index].Down, clients[index].Multiplier)
		if clients[index].Volume > 0 {
			clients[index].Remaining = clients[index].Volume - clients[index].Adjusted
			if clients[index].Remaining < 0 {
				clients[index].Remaining = 0
			}
		}
		if clients[index].Blocked {
			clients[index].Status = clients[index].BlockedReason
		} else {
			clients[index].Status = "active"
		}
	}
	return clients, nil
}

func (s *ClientService) Save(tx *gorm.DB, changes []model.Changes) error {
	var err error
	for _, change := range changes {
		client := model.Client{}
		err = json.Unmarshal(change.Obj, &client)
		if err != nil {
			return err
		}
		switch change.Action {
		case "new":
			err = tx.Create(&client).Error
		case "del":
			err = tx.Where("id = ?", change.Index).Delete(model.Client{}).Error
		default:
			err = tx.Save(client).Error
		}
		if err != nil {
			return err
		}
	}
	return err
}

type configClient struct {
	Name     string
	Config   json.RawMessage
	Inbounds []string
}

// SyncFromConfig reads the currently activated configuration. Existing traffic
// and management metadata are preserved in SQLite.
func (s *ClientService) SyncFromConfig() error {
	configPath := config.GetBinFolderPath() + "/config.active.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = config.GetBinFolderPath() + "/config.json"
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	clean, err := util.StripJSONComments(data)
	if err != nil {
		return err
	}
	var root struct {
		Inbounds []map[string]interface{} `json:"inbounds"`
	}
	if err = json.Unmarshal(clean, &root); err != nil {
		return err
	}

	found := make(map[string]*configClient)
	for _, inbound := range root.Inbounds {
		tag, _ := inbound["tag"].(string)
		users, exists := inbound["users"]
		if !exists {
			continue
		}
		userList, ok := users.([]interface{})
		if !ok {
			continue
		}
		for _, rawUser := range userList {
			user, ok := rawUser.(map[string]interface{})
			if !ok {
				continue
			}
			name := configUserName(user)
			if name == "" {
				continue
			}
			encoded, marshalErr := json.Marshal(user)
			if marshalErr != nil {
				return marshalErr
			}
			item, exists := found[name]
			if !exists {
				item = &configClient{Name: name, Config: encoded}
				found[name] = item
			}
			if tag != "" && !containsString(item.Inbounds, tag) {
				item.Inbounds = append(item.Inbounds, tag)
			}
		}
	}

	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Client{}).Where("in_config = ?", true).Update("in_config", false).Error; err != nil {
			return err
		}
		names := make([]string, 0, len(found))
		for name := range found {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			item := found[name]
			inbounds, marshalErr := json.Marshal(item.Inbounds)
			if marshalErr != nil {
				return marshalErr
			}
			var client model.Client
			err := tx.Where("name = ?", name).First(&client).Error
			if err == gorm.ErrRecordNotFound {
				client = model.Client{
					Enable:     true,
					Name:       name,
					Config:     item.Config,
					Inbounds:   inbounds,
					Links:      json.RawMessage("[]"),
					Multiplier: 1,
					InConfig:   true,
				}
				if err = tx.Create(&client).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			updates := map[string]interface{}{
				"config":    item.Config,
				"inbounds":  inbounds,
				"in_config": true,
			}
			if client.Multiplier <= 0 {
				updates["multiplier"] = 1.0
			}
			if err = tx.Model(&model.Client{}).Where("id = ?", client.Id).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func configUserName(user map[string]interface{}) string {
	for _, key := range []string{"name", "username", "email"} {
		if value, ok := user[key].(string); ok && value != "" {
			return value
		}
	}
	// Some manually written configs omit a display name. UUID is stable enough
	// to keep traffic accounting usable, although adding name is recommended.
	if value, ok := user["uuid"].(string); ok && value != "" {
		return value
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *ClientService) UpdateMetadata(id uint, enable bool, volume int64, multiplier float64, expiry int64, resetDay int, desc string) error {
	if multiplier <= 0 || multiplier > 1000 {
		return fmt.Errorf("multiplier must be greater than 0 and no more than 1000")
	}
	if volume < 0 {
		return fmt.Errorf("traffic limit cannot be negative")
	}
	if resetDay < 0 || resetDay > 31 {
		return fmt.Errorf("reset day must be between 0 and 31")
	}
	if expiry < 0 {
		return fmt.Errorf("expiry cannot be negative")
	}
	result := database.GetDB().Model(&model.Client{}).Where("id = ? AND in_config = ?", id, true).Updates(map[string]interface{}{
		"enable":     enable,
		"volume":     volume,
		"multiplier": multiplier,
		"expiry":     expiry,
		"reset_day":  resetDay,
		"desc":       desc,
	})
	if result.Error == nil && result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	if result.Error == nil {
		LastUpdate = time.Now().Unix()
	}
	return result.Error
}

func (s *ClientService) ResetTraffic(id uint) error {
	now := time.Now().Unix()
	result := database.GetDB().Model(&model.Client{}).Where("id = ? AND in_config = ?", id, true).Updates(map[string]interface{}{
		"up":            0,
		"down":          0,
		"last_reset_at": now,
	})
	if result.Error == nil && result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	if result.Error == nil {
		LastUpdate = now
	}
	return result.Error
}

// ResetDueClients resets each user once in the configured calendar month. If a
// reset day does not exist in a short month, the month's last day is used.
func (s *ClientService) ResetDueClients(now time.Time) (bool, error) {
	var clients []model.Client
	if err := database.GetDB().Where("in_config = ? AND reset_day > 0", true).Find(&clients).Error; err != nil {
		return false, err
	}
	resetAny := false
	for _, client := range clients {
		lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
		targetDay := client.ResetDay
		if targetDay > lastDay {
			targetDay = lastDay
		}
		// Use >= so a reset still happens after panel downtime on the exact day.
		if now.Day() < targetDay {
			continue
		}
		if client.LastResetAt > 0 {
			lastReset := time.Unix(client.LastResetAt, 0).In(now.Location())
			if lastReset.Year() == now.Year() && lastReset.Month() == now.Month() {
				continue
			}
		}
		if err := s.ResetTraffic(client.Id); err != nil {
			return resetAny, err
		}
		resetAny = true
		logger.Infof("monthly traffic reset for user %s", client.Name)
	}
	return resetAny, nil
}
