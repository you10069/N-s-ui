package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"s-ui/config"
	"s-ui/database"
	"s-ui/database/model"
	"s-ui/logger"
	"s-ui/singbox"
	"s-ui/util"
	"sync"
	"time"
)

const (
	ClientBlockReasonManual  = "manual"
	ClientBlockReasonExpired = "expired"
	ClientBlockReasonQuota   = "quota"
)

var runtimeConfigMu sync.Mutex

type RuntimeConfigService struct {
	singbox.Controller
	ClientService
}

func NewRuntimeConfigService() *RuntimeConfigService {
	return &RuntimeConfigService{}
}

// Reconcile evaluates all user limits and rebuilds config.runtime.json when
// necessary. force also resynchronizes user metadata from the active config.
// When restart is true, an existing
// sing-box process is fully restarted if the runtime configuration changed.
func (s *RuntimeConfigService) Reconcile(force bool, restart bool) (bool, error) {
	runtimeConfigMu.Lock()
	defer runtimeConfigMu.Unlock()

	sourcePath := s.Controller.GetActiveConfigPath()
	runtimePath := s.Controller.GetRuntimeConfigPath()

	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, err
	}
	runtimeInfo, runtimeErr := os.Stat(runtimePath)
	if runtimeErr != nil && !os.IsNotExist(runtimeErr) {
		return false, runtimeErr
	}
	sourceChanged := force || runtimeErr != nil || sourceInfo.ModTime().After(runtimeInfo.ModTime())
	if sourceChanged {
		if err = s.ClientService.SyncFromConfig(); err != nil {
			return false, err
		}
	}

	blockedNames, stateChanged, err := s.evaluateClientStates(time.Now())
	if err != nil {
		return false, err
	}
	runtimeContent, err := buildRuntimeConfig(sourcePath, blockedNames)
	if err != nil {
		return false, err
	}

	if current, readErr := os.ReadFile(runtimePath); readErr == nil && bytes.Equal(current, runtimeContent) {
		if stateChanged {
			LastUpdate = time.Now().Unix()
		}
		return false, nil
	}

	if err = s.validateRuntimeConfig(runtimeContent); err != nil {
		return false, err
	}
	if err = atomicWriteFile(runtimePath, runtimeContent, 0644); err != nil {
		return false, err
	}

	logger.Infof("runtime config updated; blocked users: %d", len(blockedNames))
	if restart && s.Controller.IsRunning() {
		if err = s.Controller.Restart(); err != nil {
			return true, err
		}
	}
	LastUpdate = time.Now().Unix()
	return true, nil
}

func (s *RuntimeConfigService) evaluateClientStates(now time.Time) (map[string]bool, bool, error) {
	var clients []model.Client
	if err := database.GetDB().Where("in_config = ?", true).Find(&clients).Error; err != nil {
		return nil, false, err
	}

	blockedNames := make(map[string]bool)
	stateChanged := false
	nowUnix := now.Unix()

	db := database.GetDB()
	for _, client := range clients {
		reason := clientLimitReason(client, nowUnix)
		blocked := reason != ""
		blockedAt := client.BlockedAt
		if blocked {
			blockedNames[client.Name] = true
			if !client.Blocked || client.BlockedReason != reason || blockedAt == 0 {
				blockedAt = nowUnix
			}
		} else {
			blockedAt = 0
		}

		if client.Blocked == blocked && client.BlockedReason == reason && client.BlockedAt == blockedAt {
			continue
		}
		stateChanged = true
		if err := db.Model(&model.Client{}).Where("id = ?", client.Id).Updates(map[string]interface{}{
			"blocked":        blocked,
			"blocked_reason": reason,
			"blocked_at":     blockedAt,
		}).Error; err != nil {
			return nil, false, err
		}
	}
	return blockedNames, stateChanged, nil
}

func clientLimitReason(client model.Client, nowUnix int64) string {
	if !client.Enable {
		return ClientBlockReasonManual
	}
	if client.Expiry > 0 && nowUnix >= client.Expiry {
		return ClientBlockReasonExpired
	}
	if client.Volume > 0 && adjustedTraffic(client.Up, client.Down, client.Multiplier) >= client.Volume {
		return ClientBlockReasonQuota
	}
	return ""
}

func adjustedTraffic(up int64, down int64, multiplier float64) int64 {
	if multiplier <= 0 {
		multiplier = 1
	}
	value := float64(up+down) * multiplier
	if value >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	if value <= 0 {
		return 0
	}
	return int64(value)
}

func buildRuntimeConfig(sourcePath string, blockedNames map[string]bool) ([]byte, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, err
	}
	clean, err := util.StripJSONComments(data)
	if err != nil {
		return nil, err
	}

	var root map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(clean))
	decoder.UseNumber()
	if err = decoder.Decode(&root); err != nil {
		return nil, err
	}
	filterBlockedUsers(root, blockedNames)

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(root); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// filterBlockedUsers removes blocked users from each inbound. If an inbound
// originally required users and all of them are blocked, the entire inbound is
// removed. Keeping an empty users array could turn SOCKS/HTTP authentication
// off and accidentally expose an unauthenticated proxy.
func filterBlockedUsers(root map[string]interface{}, blockedNames map[string]bool) {
	inbounds, ok := root["inbounds"].([]interface{})
	if !ok || len(blockedNames) == 0 {
		return
	}

	filteredInbounds := make([]interface{}, 0, len(inbounds))
	for _, rawInbound := range inbounds {
		inbound, ok := rawInbound.(map[string]interface{})
		if !ok {
			filteredInbounds = append(filteredInbounds, rawInbound)
			continue
		}
		rawUsers, hasUsers := inbound["users"]
		users, usersOK := rawUsers.([]interface{})
		if !hasUsers || !usersOK || len(users) == 0 {
			filteredInbounds = append(filteredInbounds, rawInbound)
			continue
		}

		activeUsers := make([]interface{}, 0, len(users))
		for _, rawUser := range users {
			user, ok := rawUser.(map[string]interface{})
			if !ok {
				activeUsers = append(activeUsers, rawUser)
				continue
			}
			name := configUserName(user)
			if name != "" && blockedNames[name] {
				continue
			}
			activeUsers = append(activeUsers, rawUser)
		}
		if len(activeUsers) == 0 {
			continue
		}
		inbound["users"] = activeUsers
		filteredInbounds = append(filteredInbounds, inbound)
	}
	root["inbounds"] = filteredInbounds
}

func (s *RuntimeConfigService) validateRuntimeConfig(content []byte) error {
	configDir := config.GetBinFolderPath()
	tempFile, err := os.CreateTemp(configDir, ".runtime-check-*.json")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err = tempFile.Write(content); err != nil {
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
		return fmt.Errorf("runtime sing-box check failed: %s", message)
	}
	return nil
}

func atomicWriteFile(path string, data []byte, defaultMode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	mode := defaultMode
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tempFile, err := os.CreateTemp(dir, ".runtime-save-*.json")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if err = tempFile.Chmod(mode); err != nil {
		tempFile.Close()
		return err
	}
	if _, err = tempFile.Write(data); err != nil {
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
	return os.Rename(tempPath, path)
}
