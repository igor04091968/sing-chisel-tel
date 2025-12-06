package api

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"
	"github.com/alireza0/s-ui/logger"
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util"

	"github.com/gin-gonic/gin"
)

type ApiService struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.TlsService
	service.InboundService
	service.OutboundService
	service.EndpointService
	service.ServicesService
	service.PanelService
	service.StatsService
	service.ServerService
	service.ChiselService
	service.GostService
	service.MTProtoService
	service.GreService
	service.TapService
	service.UdpTunnelService
}

func (a *ApiService) LoadData(c *gin.Context) {
	data, err := a.getData(c)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, nil)
}

func (a *ApiService) getData(c *gin.Context) (interface{}, error) {
	data := make(map[string]interface{}, 0)
	lu := c.Query("lu")
	isUpdated, err := a.ConfigService.CheckChanges(lu)
	if err != nil {
		return "", err
	}
	onlines, err := a.StatsService.GetOnlines()

	sysInfo := a.ServerService.GetSingboxInfo()
	if sysInfo["running"] == false {
		logs := a.ServerService.GetLogs("1", "debug")
		if len(logs) > 0 {
			data["lastLog"] = logs[0]
		}
	}

	if err != nil {
		return "", err
	}
	if isUpdated {
		config, err := a.SettingService.GetConfig()
		if err != nil {
			return "", err
		}
		clients, err := a.ClientService.GetAllUsers()
		if err != nil {
			return "", err
		}
		tlsConfigs, err := a.TlsService.GetAll()
		if err != nil {
			return "", err
		}
		inbounds, err := a.InboundService.GetAll()
		if err != nil {
			return "", err
		}
		outbounds, err := a.OutboundService.GetAll()
		if err != nil {
			return "", err
		}
		endpoints, err := a.EndpointService.GetAll()
		if err != nil {
			return "", err
		}
		services, err := a.ServicesService.GetAll()
		if err != nil {
			return "", err
		}
		chiselConfigs, err := a.ChiselService.GetAllChiselConfigs()
		if err != nil {
			return "", err
		}
		mtprotoConfigs, err := a.MTProtoService.GetAllMTProtoProxies()
		if err != nil {
			return "", err
		}
		greConfigs, err := a.GreService.GetAllGreTunnels()
		if err != nil {
			return "", err
		}
		tapConfigs, err := a.TapService.GetAllTapTunnels()
		if err != nil {
			return "", err
		}
		udpTunnelConfigs, err := a.UdpTunnelService.GetAllUdpTunnels()
		if err != nil {
			return "", err
		}
		subURI, err := a.SettingService.GetFinalSubURI(getHostname(c))
		if err != nil {
			return "", err
		}
		trafficAge, err := a.SettingService.GetTrafficAge()
		if err != nil {
			return "", err
		}
		data["config"] = json.RawMessage(config)
		data["clients"] = clients
		data["tls"] = tlsConfigs
		data["inbounds"] = inbounds
		data["outbounds"] = outbounds
		data["endpoints"] = endpoints
		data["services"] = services
		data["chisel"] = chiselConfigs
		data["mtproto"] = mtprotoConfigs
		data["gre"] = greConfigs
		data["tap"] = tapConfigs
		data["udptunnel"] = udpTunnelConfigs
		data["subURI"] = subURI
		data["enableTraffic"] = trafficAge > 0
		data["onlines"] = onlines
	} else {
		data["onlines"] = onlines
	}

	return data, nil
}

func (a *ApiService) LoadPartialData(c *gin.Context, objs []string) error {
	data := make(map[string]interface{}, 0)
	id := c.Query("id")

	for _, obj := range objs {
		switch obj {
		case "inbounds":
			inbounds, err := a.InboundService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = inbounds
		case "outbounds":
			outbounds, err := a.OutboundService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = outbounds
		case "endpoints":
			endpoints, err := a.EndpointService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = endpoints
		case "services":
			services, err := a.ServicesService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = services
		case "chisel":
			chiselConfigs, err := a.ChiselService.GetAllChiselConfigs()
			if err != nil {
				return err
			}
			data[obj] = chiselConfigs
		case "mtproto":
			mtprotoConfigs, err := a.MTProtoService.GetAllMTProtoProxies()
			if err != nil {
				return err
			}
			data[obj] = mtprotoConfigs
		case "gre":
			greConfigs, err := a.GreService.GetAllGreTunnels()
			if err != nil {
				return err
			}
			data[obj] = greConfigs
		case "tap":
			tapConfigs, err := a.TapService.GetAllTapTunnels()
			if err != nil {
				return err
			}
			data[obj] = tapConfigs
		case "udptunnel":
			udpTunnelConfigs, err := a.UdpTunnelService.GetAllUdpTunnels()
			if err != nil {
				return err
			}
			data[obj] = udpTunnelConfigs
		case "tls":
			tlsConfigs, err := a.TlsService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = tlsConfigs
		case "clients":
			clients, err := a.ClientService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = clients
		case "config":
			config, err := a.SettingService.GetConfig()
			if err != nil {
				return err
			}
			data[obj] = json.RawMessage(config)
		case "settings":
			settings, err := a.SettingService.GetAllSetting()
			if err != nil {
				return err
			}
			data[obj] = settings
		}
	}

	jsonObj(c, data, nil)
	return nil
}

func (a *ApiService) GetUsers(c *gin.Context) {
	users, err := a.ClientService.GetAllUsers()
	if err != nil {
		jsonMsg(c, "users", err)
		return
	}
	jsonObj(c, users, nil)
}

func (a *ApiService) GetSettings(c *gin.Context) {
	data, err := a.SettingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStats(c *gin.Context) {
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
}

func (a *ApiService) GetStatus(c *gin.Context) {
	request := c.Query("r")
	result := a.ServerService.GetStatus(request)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetOnlines(c *gin.Context) {
	onlines, err := a.StatsService.GetOnlines()
	jsonObj(c, onlines, err)
}

func (a *ApiService) GetLogs(c *gin.Context) {
	count := c.Query("c")
	level := c.Query("l")
	logs := a.ServerService.GetLogs(count, level)
	jsonObj(c, logs, nil)
}

func (a *ApiService) CheckChanges(c *gin.Context) {
	actor := c.Query("a")
	chngKey := c.Query("k")
	count := c.Query("c")
	changes := a.ConfigService.GetChanges(actor, chngKey, count)
	jsonObj(c, changes, nil)
}

func (a *ApiService) GetKeypairs(c *gin.Context) {
	kType := c.Query("k")
	options := c.Query("o")
	keypair := a.ServerService.GenKeypair(kType, options)
	jsonObj(c, keypair, nil)
}

func (a *ApiService) GetDb(c *gin.Context) {
	exclude := c.Query("exclude")
	db, err := database.GetDb(exclude)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=s-ui_"+time.Now().Format("20060102-150405")+".db")
	c.Writer.Write(db)
}

// postActions is a helper function to extract action and data from a POST request.
func (a *ApiService) postActions(c *gin.Context) (string, json.RawMessage, error) {
	var data map[string]json.RawMessage
	err := c.ShouldBind(&data)
	if err != nil {
		return "", nil, err
	}
	return string(data["action"]), data["data"], nil
}

// APIHandler struct (assuming it's defined elsewhere, e.g., in apiHandler.go)
// For the purpose of this modification, we'll assume a.ApiService is available.
// This function is not part of ApiService, but rather a handler that uses ApiService.
// We need to find the actual postHandler function in api/apiHandler.go

// Login handles user login requests.
func (a *ApiService) Login(c *gin.Context) {
	remoteIP := getRemoteIp(c)
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
}

// ChangePass handles password change requests.
func (a *ApiService) ChangePass(c *gin.Context) {
	id := c.Request.FormValue("id")
	oldPass := c.Request.FormValue("oldPass")
	newUsername := c.Request.FormValue("newUsername")
	newPass := c.Request.FormValue("newPass")
	err := a.UserService.ChangePass(id, oldPass, newUsername, newPass)
	if err == nil {
		jsonMsg(c, "save", nil)
	} else {
		logger.Warning("change user credentials failed:", err)
		jsonMsg(c, "", err)
	}
}

// Save handles saving various configurations.
func (a *ApiService) Save(c *gin.Context, loginUser string) {
	hostname := getHostname(c)
	obj := c.Request.FormValue("object")
	act := c.Request.FormValue("action")
	data := c.Request.FormValue("data")
	initUsers := c.Request.FormValue("initUsers")
	objs, err := a.ConfigService.Save(obj, act, json.RawMessage(data), initUsers, loginUser, hostname)
	if err != nil {
		jsonMsg(c, "save", err)
		return
	}
	err = a.LoadPartialData(c, objs)
	if err != nil {
		jsonMsg(c, obj, err)
	}
}

// RestartApp restarts the panel application.
func (a *ApiService) RestartApp(c *gin.Context) {
	err := a.PanelService.RestartPanel(3)
	jsonMsg(c, "restartApp", err)
}

// RestartSb restarts the sing-box core.
func (a *ApiService) RestartSb(c *gin.Context) {
	err := a.ConfigService.RestartCore()
	jsonMsg(c, "restartSb", err)
}

// LinkConvert converts a link.
func (a *ApiService) LinkConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, _, err := util.GetOutbound(link, 0)
	jsonObj(c, result, err)
}

// ImportDb imports a database file.
func (a *ApiService) ImportDb(c *gin.Context) {
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	defer file.Close()
	err = database.ImportDB(file)
	jsonMsg(c, "", err)
}

// Logout handles user logout requests.
func (a *ApiService) Logout(c *gin.Context) {
	loginUser := GetLoginUser(c)
	if loginUser != "" {
		logger.Infof("user %s logout", loginUser)
	}
	ClearSession(c)
	jsonMsg(c, "", nil)
}

// LoadTokens loads user tokens.
func (a *ApiService) LoadTokens() ([]byte, error) {
	return a.UserService.LoadTokens()
}

// GetTokens retrieves user tokens.
func (a *ApiService) GetTokens(c *gin.Context) {
	loginUser := GetLoginUser(c)
	tokens, err := a.UserService.GetUserTokens(loginUser)
	jsonObj(c, tokens, err)
}

// AddToken adds a new user token.
func (a *ApiService) AddToken(c *gin.Context) {
	loginUser := GetLoginUser(c)
	expiry := c.Request.FormValue("expiry")
	expiryInt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	desc := c.Request.FormValue("desc")
	token, err := a.UserService.AddToken(loginUser, expiryInt, desc)
	jsonObj(c, token, err)
}

// DeleteToken deletes a user token.
func (a *ApiService) DeleteToken(c *gin.Context) {
	tokenId := c.Request.FormValue("id")
	err := a.UserService.DeleteToken(tokenId)
	jsonMsg(c, "", err)
}

// GOST API methods
func (a *ApiService) GetGosts(c *gin.Context) {
	configs, err := a.GostService.GetAllGostConfigs()
	if err != nil {
		jsonMsg(c, "gosts", err)
		return
	}
	jsonObj(c, map[string]interface{}{"gosts": configs}, nil)
}

func (a *ApiService) SaveGost(c *gin.Context) {
	action := c.Request.FormValue("action")
	data := c.Request.FormValue("data")
	if action == "new" {
		var cfg model.GostConfig
		if err := json.Unmarshal([]byte(data), &cfg); err != nil {
			jsonMsg(c, "gost_save", err)
			return
		}
		if err := a.GostService.CreateGostConfig(&cfg); err != nil {
			jsonMsg(c, "gost_save", err)
			return
		}
		jsonMsg(c, "gost_save", nil)
	}
}

func (a *ApiService) UpdateGost(c *gin.Context) {
	data := c.Request.FormValue("data")
	var cfg model.GostConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		jsonMsg(c, "gost_update", err)
		return
	}
	var orig model.GostConfig
	if err := database.GetDB().Model(&orig).Where("id = ?", cfg.ID).First(&orig).Error; err != nil {
		jsonMsg(c, "gost_update", err)
		return
	}
	if err := database.GetDB().Model(&orig).Updates(&cfg).Error; err != nil {
		jsonMsg(c, "gost_update", err)
		return
	}
	jsonMsg(c, "gost_update", nil)
}

func (a *ApiService) StartGost(c *gin.Context) {
	data := c.Request.FormValue("data")
	var cfg model.GostConfig
	if err := json.Unmarshal([]byte(data), &cfg); err != nil {
		jsonMsg(c, "gost_start", err)
		return
	}
	var orig model.GostConfig
	if err := database.GetDB().Model(&orig).Where("id = ?", cfg.ID).First(&orig).Error; err != nil {
		jsonMsg(c, "gost_start", err)
		return
	}
	if err := a.GostService.StartGost(&orig); err != nil {
		jsonMsg(c, "gost_start", err)
		return
	}
	jsonMsg(c, "gost_start", nil)
}

func (a *ApiService) StopGost(c *gin.Context) {
	idStr := c.Request.FormValue("id")
	idI, err := strconv.Atoi(idStr)
	if err != nil {
		jsonMsg(c, "gost_stop", err)
		return
	}
	if err := a.GostService.StopGost(uint(idI)); err != nil {
		jsonMsg(c, "gost_stop", err)
		return
	}
	jsonMsg(c, "gost_stop", nil)
}

func (a *ApiService) DeleteGost(c *gin.Context) {
	idStr := c.Request.FormValue("id")
	idI, err := strconv.Atoi(idStr)
	if err != nil {
		jsonMsg(c, "gost_delete", err)
		return
	}
	if err := a.GostService.DeleteGostConfig(uint(idI)); err != nil {
		jsonMsg(c, "gost_delete", err)
		return
	}
	jsonMsg(c, "gost_delete", nil)
}