package api

import (
	"s-ui/logger"
	"s-ui/service"
	"s-ui/singbox"
	"s-ui/util"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type configTextRequest struct {
	Content string `json:"content" binding:"required"`
}

type clientMetadataRequest struct {
	ID         uint    `json:"id" binding:"required"`
	Enable     bool    `json:"enable"`
	Volume     int64   `json:"volume"`
	Multiplier float64 `json:"multiplier"`
	Expiry     int64   `json:"expiry"`
	ResetDay   int     `json:"resetDay"`
	Desc       string  `json:"desc"`
}

type clientResetRequest struct {
	ID uint `json:"id" binding:"required"`
}

type APIHandler struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.TlsService
	service.InDataService
	service.PanelService
	service.StatsService
	service.ServerService
	singbox.Controller
}

func NewAPIHandler(g *gin.RouterGroup) {
	a := &APIHandler{}
	a.initRouter(g)
}

func (a *APIHandler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasSuffix(path, "login") && !strings.HasSuffix(path, "logout") {
			checkLogin(c)
		}
	})

	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIHandler) postHandler(c *gin.Context) {
	var err error
	action := c.Param("postAction")
	remoteIP := getRemoteIp(c)

	switch action {
	case "login":
		loginUser, err := a.UserService.Login(c.Request.FormValue("user"), c.Request.FormValue("pass"), remoteIP)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}

		sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
		if err != nil {
			logger.Infof("Unable to get session's max age from DB")
		}

		err = SetLoginUser(c, loginUser, sessionMaxAge)
		if err == nil {
			logger.Info("user ", loginUser, " login success")
		} else {
			logger.Warning("login failed: ", err)
		}

		jsonMsg(c, "", nil)
	case "changePass":
		id := c.Request.FormValue("id")
		oldPass := c.Request.FormValue("oldPass")
		newUsername := c.Request.FormValue("newUsername")
		newPass := c.Request.FormValue("newPass")
		err = a.UserService.ChangePass(id, oldPass, newUsername, newPass)
		if err == nil {
			logger.Info("change user credentials success")
			jsonMsg(c, "save", nil)
		} else {
			logger.Warning("change user credentials failed:", err)
			jsonMsg(c, "", err)
		}
	case "save":
		loginUser := GetLoginUser(c)
		data := map[string]string{}
		err = c.ShouldBind(&data)
		if err == nil {
			err = a.ConfigService.SaveChanges(data, loginUser)
		}
		jsonMsg(c, "save", err)
	case "restartApp":
		err = a.PanelService.RestartPanel(3)
		jsonMsg(c, "restartApp", err)
	case "restartSb":
		err = a.ConfigService.ReloadCore()
		jsonMsg(c, "restartSb", err)
	case "checkConfigText":
		var request configTextRequest
		if err = c.ShouldBindJSON(&request); err == nil {
			err = a.ConfigService.CheckRawConfig(request.Content)
		}
		jsonMsg(c, "check", err)
	case "saveConfigText":
		var request configTextRequest
		if err = c.ShouldBindJSON(&request); err == nil {
			err = a.ConfigService.SaveRawConfig(request.Content)
		}
		jsonMsg(c, "save", err)
	case "updateClient":
		var request clientMetadataRequest
		if err = c.ShouldBindJSON(&request); err == nil {
			err = a.ClientService.UpdateMetadata(request.ID, request.Enable, request.Volume, request.Multiplier, request.Expiry, request.ResetDay, request.Desc)
		}
		if err == nil {
			_, err = service.NewRuntimeConfigService().Reconcile(false, true)
		}
		jsonMsg(c, "save", err)
	case "resetClientTraffic":
		var request clientResetRequest
		if err = c.ShouldBindJSON(&request); err == nil {
			err = a.ClientService.ResetTraffic(request.ID)
		}
		if err == nil {
			_, err = service.NewRuntimeConfigService().Reconcile(false, true)
		}
		jsonMsg(c, "reset", err)
	case "linkConvert":
		link := c.Request.FormValue("link")
		result, _, err := util.GetOutbound(link, 0)
		jsonObj(c, result, err)
	default:
		jsonMsg(c, "API call", nil)
	}
}

func (a *APIHandler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "logout":
		loginUser := GetLoginUser(c)
		if loginUser != "" {
			logger.Infof("user %s logout", loginUser)
		}
		ClearSession(c)
		jsonMsg(c, "", nil)
	case "load":
		data, err := a.loadData(c)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, data, nil)
	case "users":
		users, err := a.UserService.GetUsers()
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, *users, nil)
	case "clients":
		if err := a.ClientService.SyncFromConfig(); err != nil {
			jsonMsg(c, "", err)
			return
		}
		clients, err := a.ClientService.GetAll()
		jsonObj(c, clients, err)
	case "configText":
		content, updatedAt, err := a.ConfigService.GetRawConfig()
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		pendingReload, err := a.ConfigService.IsReloadPending()
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, gin.H{"content": content, "updatedAt": updatedAt, "pendingReload": pendingReload}, nil)
	case "runtimeConfigText":
		content, updatedAt, exists, err := a.ConfigService.GetRuntimeConfig()
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, gin.H{
			"content":   content,
			"updatedAt": updatedAt,
			"exists":    exists,
			"running":   a.Controller.IsRunning(),
		}, nil)
	case "setting":
		data, err := a.SettingService.GetAllSetting()
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, data, err)
	case "stats":
		resource := c.Query("resource")
		tag := c.Query("tag")
		limit, err := strconv.Atoi(c.Query("limit"))
		if err != nil {
			limit = 100
		}
		data, err := a.StatsService.GetStats(resource, tag, limit)
		if err != nil {
			jsonMsg(c, "", err)
			return
		}
		jsonObj(c, data, err)
	case "status":
		request := c.Query("r")
		result := a.ServerService.GetStatus(request)
		jsonObj(c, result, nil)
	case "onlines":
		onlines, err := a.StatsService.GetOnlines()
		jsonObj(c, onlines, err)
	case "logs":
		service := c.Query("s")
		count := c.Query("c")
		level := c.Query("l")
		logs := a.ServerService.GetLogs(service, count, level)
		jsonObj(c, logs, nil)
	case "changes":
		actor := c.Query("a")
		chngKey := c.Query("k")
		count := c.Query("c")
		changes := a.ConfigService.GetChanges(actor, chngKey, count)
		jsonObj(c, changes, nil)
	case "keypairs":
		kType := c.Query("k")
		options := c.Query("o")
		keypair := a.ServerService.GenKeypair(kType, options)
		jsonObj(c, keypair, nil)
	default:
		jsonMsg(c, "API call", nil)
	}
}

func (a *APIHandler) loadData(c *gin.Context) (interface{}, error) {
	data := make(map[string]interface{}, 0)
	onlines, err := a.StatsService.GetOnlines()
	if err != nil {
		return "", err
	}
	data["onlines"] = onlines

	sysInfo := a.ServerService.GetSingboxInfo()
	if sysInfo["running"] == false {
		logs := a.ServerService.GetLogs("sing-box", "1", "debug")
		if len(logs) > 0 {
			data["lastLog"] = logs[0]
		}
	}
	return data, nil
}
