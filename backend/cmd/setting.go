package cmd

import (
	"fmt"
	"s-ui/config"
	"s-ui/database"
	"s-ui/service"
)

func resetSetting() {
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Println(err)
		return
	}

	settingService := service.SettingService{}
	if err := settingService.ResetSettings(); err != nil {
		fmt.Println("reset setting failed:", err)
	} else {
		fmt.Println("reset setting success")
	}
}

func updateSetting(port int, path string) {
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Println(err)
		return
	}

	settingService := service.SettingService{}
	if port > 0 {
		if err := settingService.SetPort(port); err != nil {
			fmt.Println("set port failed:", err)
		} else {
			fmt.Println("set port success")
		}
	}
	if path != "" {
		if err := settingService.SetWebPath(path); err != nil {
			fmt.Println("set path failed:", err)
		} else {
			fmt.Println("set path success")
		}
	}
}

func showSetting() {
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Println(err)
		return
	}
	settingService := service.SettingService{}
	allSetting, err := settingService.GetAllSetting()
	if err != nil {
		fmt.Println("get current settings failed, error info:", err)
		return
	}
	fmt.Println("Current panel settings:")
	fmt.Println("\tPanel port:\t", (*allSetting)["webPort"])
	fmt.Println("\tPanel path:\t", (*allSetting)["webPath"])
	if (*allSetting)["webListen"] != "" {
		fmt.Println("\tPanel IP:\t", (*allSetting)["webListen"])
	}
	if (*allSetting)["webDomain"] != "" {
		fmt.Println("\tPanel Domain:\t", (*allSetting)["webDomain"])
	}
	if (*allSetting)["webURI"] != "" {
		fmt.Println("\tPanel URI:\t", (*allSetting)["webURI"])
	}
}
