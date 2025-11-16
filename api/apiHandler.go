package api

import (
	"strings"

	"github.com/alireza0/s-ui/util/common"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	ApiService
	apiv2 *APIv2Handler
}

func NewAPIHandler(g *gin.RouterGroup, a2 *APIv2Handler) {
	a := &APIHandler{
		apiv2: a2,
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
	case "mtprotos":
		a.ApiService.GetMTProtos(c)
	case "gres":
		a.ApiService.GetGres(c)
	case "taps":
		a.ApiService.GetTaps(c)
	case "mtproto_save":
		a.ApiService.SaveMTProto(c)
	case "mtproto_start":
		a.ApiService.StartMTProto(c)
	case "mtproto_stop":
		a.ApiService.StopMTProto(c)
	case "mtproto_delete":
		a.ApiService.DeleteMTProto(c)
	case "mtproto_update":
		a.ApiService.UpdateMTProto(c)
	case "gre_save":
		a.ApiService.SaveGre(c)
	case "gre_start":
		a.ApiService.StartGre(c)
	case "gre_stop":
		a.ApiService.StopGre(c)
	case "gre_delete":
		a.ApiService.DeleteGre(c)
	case "gre_update":
		a.ApiService.UpdateGre(c)
	case "tap_save":
		a.ApiService.SaveTap(c)
	case "tap_start":
		a.ApiService.StartTap(c)
	case "tap_stop":
		a.ApiService.StopTap(c)
	case "tap_delete":
		a.ApiService.DeleteTap(c)
	case "tap_update":
		a.ApiService.UpdateTap(c)
	case "gost_update":
		a.ApiService.UpdateGost(c)
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
	case "gosts":
		a.ApiService.GetGosts(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}
