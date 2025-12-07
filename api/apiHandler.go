package api

import (
	"strconv"
	"strings"

	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/igor04091968/sing-chisel-tel/service"
	"github.com/igor04091968/sing-chisel-tel/util/common"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	ApiService
	apiv2 *APIv2Handler
}

// NewAPIHandler creates API routes and initializes ApiService from provided bundle.
func NewAPIHandler(g *gin.RouterGroup, a2 *APIv2Handler, bundle *service.ServicesBundle) {
	a := &APIHandler{
		apiv2: a2,
	}

	// If a services bundle is provided, populate the embedded ApiService fields
	if bundle != nil {
		a.ApiService.SettingService = bundle.SettingService
		a.ApiService.UserService = bundle.UserService
		if bundle.ConfigService != nil {
			a.ApiService.ConfigService = *bundle.ConfigService
		}
		a.ApiService.ClientService = bundle.ClientService
		a.ApiService.TlsService = bundle.TlsService
		a.ApiService.InboundService = bundle.InboundService
		a.ApiService.OutboundService = bundle.OutboundService
		a.ApiService.EndpointService = bundle.EndpointService
		a.ApiService.ServicesService = bundle.ServicesService
		a.ApiService.PanelService = bundle.PanelService
		a.ApiService.StatsService = bundle.StatsService
		a.ApiService.ServerService = bundle.ServerService
		if bundle.ChiselService != nil {
			a.ApiService.ChiselService = *bundle.ChiselService
		}
		if bundle.GostService != nil {
			a.ApiService.GostService = *bundle.GostService
		}
		if bundle.UdpTunnelService != nil {
			a.ApiService.UdpTunnelService = *bundle.UdpTunnelService
		}
	}

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
	loginUser := GetLoginUser(c)
	action := c.Param("postAction")

	switch action {
	case "login":
		a.ApiService.Login(c)
	case "changePass":
		a.ApiService.ChangePass(c)
	case "save":
		a.ApiService.Save(c, loginUser)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "gost_save":
		a.ApiService.SaveGost(c)
	case "gost_start":
		a.ApiService.StartGost(c)
	case "gost_stop":
		a.ApiService.StopGost(c)
	case "gost_delete":
		a.ApiService.DeleteGost(c)
	case "gost_update":
		a.ApiService.UpdateGost(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	case "addToken":
		a.ApiService.AddToken(c)
		a.apiv2.ReloadTokens()
	case "deleteToken":
		a.ApiService.DeleteToken(c)
		a.apiv2.ReloadTokens()
	case "mtproto_save":
		var config model.MTProtoProxyConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			jsonMsg(c, "mtproto_save", err)
			return
		}
		err := a.ApiService.CreateMTProtoProxy(&config)
		jsonMsg(c, "mtproto_save", err)
	case "mtproto_start":
		var config model.MTProtoProxyConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			jsonMsg(c, "mtproto_start", err)
			return
		}
		err := a.ApiService.StartMTProtoProxy(&config)
		jsonMsg(c, "mtproto_start", err)
	case "mtproto_stop":
		id, err := strconv.ParseUint(c.PostForm("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "mtproto_stop", err)
			return
		}
		err = a.ApiService.StopMTProtoProxy(uint(id))
		jsonMsg(c, "mtproto_stop", err)
	case "mtproto_delete":
		id, err := strconv.ParseUint(c.PostForm("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "mtproto_delete", err)
			return
		}
		err = a.ApiService.DeleteMTProtoProxy(uint(id))
		jsonMsg(c, "mtproto_delete", err)
	case "mtproto_update":
		var config model.MTProtoProxyConfig
		if err := c.ShouldBindJSON(&config); err != nil {
			jsonMsg(c, "mtproto_update", err)
			return
		}
		err := a.ApiService.UpdateMTProtoProxy(&config)
		jsonMsg(c, "mtproto_update", err)
	case "gre_save":
		var config model.GreTunnel
		if err := c.ShouldBindJSON(&config); err != nil {
			jsonMsg(c, "gre_save", err)
			return
		}
		err := a.ApiService.CreateGreTunnel(&config)
		jsonMsg(c, "gre_save", err)
	// case "gre_start":
	// 	a.ApiService.StartGre(c)
	// case "gre_stop":
	// 	a.ApiService.StopGre(c)
	case "gre_delete":
		id, err := strconv.ParseUint(c.PostForm("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "gre_delete", err)
			return
		}
		err = a.ApiService.DeleteGreTunnel(uint(id))
		jsonMsg(c, "gre_delete", err)
	// case "gre_update":
	// 	a.ApiService.UpdateGre(c)
	case "tap_save":
		var config model.TapTunnel
		if err := c.ShouldBindJSON(&config); err != nil {
			jsonMsg(c, "tap_save", err)
			return
		}
		err := a.ApiService.CreateTapTunnel(&config)
		jsonMsg(c, "tap_save", err)
	// case "tap_start":
	// 	a.ApiService.StartTap(c)
	// case "tap_stop":
	// 	a.ApiService.StopTap(c)
	case "tap_delete":
		id, err := strconv.ParseUint(c.PostForm("id"), 10, 32)
		if err != nil {
			jsonMsg(c, "tap_delete", err)
			return
		}
		err = a.ApiService.DeleteTapTunnel(uint(id))
		jsonMsg(c, "tap_delete", err)
	// case "tap_update":
	// 	a.ApiService.UpdateTap(c)
    case "udp_tunnel_save":
        var config model.UdpTunnelConfig
        if err := c.ShouldBindJSON(&config); err != nil {
            jsonMsg(c, "udp_tunnel_save", err)
            return
        }
        err := a.ApiService.CreateUdpTunnel(&config)
        jsonMsg(c, "udp_tunnel_save", err)
    case "udp_tunnel_start":
        id, err := strconv.ParseUint(c.PostForm("id"), 10, 32)
        if err != nil {
            jsonMsg(c, "udp_tunnel_start", err)
            return
        }
        cfg, err := a.ApiService.GetUdpTunnelByID(uint(id))
        if err != nil {
            jsonMsg(c, "udp_tunnel_start", err)
            return
        }
        err = a.ApiService.StartUdpTunnel(cfg)
        jsonMsg(c, "udp_tunnel_start", err)
    case "udp_tunnel_stop":
        id, err := strconv.ParseUint(c.PostForm("id"), 10, 32)
        if err != nil {
            jsonMsg(c, "udp_tunnel_stop", err)
            return
        }
        err = a.ApiService.StopUdpTunnel(uint(id))
        jsonMsg(c, "udp_tunnel_stop", err)
    case "udp_tunnel_delete":
        id, err := strconv.ParseUint(c.PostForm("id"), 10, 32)
        if err != nil {
            jsonMsg(c, "udp_tunnel_delete", err)
            return
        }
        err = a.ApiService.DeleteUdpTunnel(uint(id))
        jsonMsg(c, "udp_tunnel_delete", err)
    case "udp_tunnel_update":
        var config model.UdpTunnelConfig
        if err := c.ShouldBindJSON(&config); err != nil {
            jsonMsg(c, "udp_tunnel_update", err)
            return
        }
        err := a.ApiService.UpdateUdpTunnel(&config)
        jsonMsg(c, "udp_tunnel_update", err)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIHandler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "logout":
		a.ApiService.Logout(c)
	case "load":
		a.ApiService.LoadData(c)
	case "inbounds", "outbounds", "endpoints", "services", "tls", "clients", "config":
		err := a.ApiService.LoadPartialData(c, []string{action})
		if err != nil {
			jsonMsg(c, action, err)
		}
		return
	case "users":
		a.ApiService.GetUsers(c)
	case "settings":
		a.ApiService.GetSettings(c)
	case "stats":
		a.ApiService.GetStats(c)
	case "status":
		a.ApiService.GetStatus(c)
	case "onlines":
		a.ApiService.GetOnlines(c)
	case "logs":
		a.ApiService.GetLogs(c)
	case "changes":
		a.ApiService.CheckChanges(c)
	case "keypairs":
		a.ApiService.GetKeypairs(c)
	case "getdb":
		a.ApiService.GetDb(c)
	case "tokens":
		a.ApiService.GetTokens(c)
	case "mtpros":
		proxies, err := a.ApiService.GetAllMTProtoProxies()
		if err != nil {
			jsonMsg(c, "mtpros", err)
			return
		}
		jsonObj(c, proxies, nil)
	case "gres":
		tunnels, err := a.ApiService.GetAllGreTunnels()
		if err != nil {
			jsonMsg(c, "gres", err)
			return
		}
		jsonObj(c, tunnels, nil)
	case "taps":
		tunnels, err := a.ApiService.GetAllTapTunnels()
		if err != nil {
			jsonMsg(c, "taps", err)
			return
		}
		jsonObj(c, tunnels, nil)
	case "udptunnels":
		tunnels, err := a.ApiService.GetAllUdpTunnels()
		if err != nil {
			jsonMsg(c, "udptunnels", err)
			return
		}
		jsonObj(c, tunnels, nil)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

