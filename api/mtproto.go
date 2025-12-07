package api

import (
	"strconv"

	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/igor04091968/sing-chisel-tel/service"
	"github.com/gin-gonic/gin"
)

// MTProtoAPI handles API requests for MTProto Proxies.
type MTProtoAPI struct {
	mtprotoService *service.MTProtoService
}

// NewMTProtoAPI creates a new instance of MTProtoAPI.
func NewMTProtoAPI() *MTProtoAPI {
	return &MTProtoAPI{
		mtprotoService: service.NewMTProtoService(),
	}
}

// RegisterRoutes registers the API routes for MTProto Proxies.
func (a *MTProtoAPI) RegisterRoutes(router *gin.RouterGroup) {
	mtprotoGroup := router.Group("/mtproto")
	mtprotoGroup.GET("", a.getMTProtoProxies)
	mtprotoGroup.POST("", a.createMTProtoProxy)
	mtprotoGroup.GET("/:id", a.getMTProtoProxy)
	mtprotoGroup.PUT("/:id", a.updateMTProtoProxy)
	mtprotoGroup.DELETE("/:id", a.deleteMTProtoProxy)
	mtprotoGroup.POST("/:id/start", a.startMTProtoProxy)
	mtprotoGroup.POST("/:id/stop", a.stopMTProtoProxy)
	mtprotoGroup.GET("/generate-secret", a.generateSecret)
}

// getMTProtoProxies godoc
// @Summary Get all MTProto Proxies
// @Description Retrieves a list of all configured MTProto Proxies.
// @Tags MTProto
// @Produce json
// @Success 200 {array} model.MTProtoProxyConfig
// @Failure 500 {object} object{message=string}
// @Router /mtproto [get]
func (a *MTProtoAPI) getMTProtoProxies(c *gin.Context) {
	configs, err := a.mtprotoService.GetAllMTProtoProxies()
	if err != nil {
		jsonMsg(c, "Failed to get MTProto Proxies", err)
		return
	}
	jsonObj(c, configs, nil)
}

// createMTProtoProxy godoc
// @Summary Create an MTProto Proxy
// @Description Creates a new MTProto Proxy instance.
// @Tags MTProto
// @Accept json
// @Produce json
// @Param config body model.MTProtoProxyConfig true "MTProto Proxy Configuration"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /mtproto [post]
func (a *MTProtoAPI) createMTProtoProxy(c *gin.Context) {
	var config model.MTProtoProxyConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "Invalid MTProto Proxy config", err)
		return
	}
	if err := a.mtprotoService.CreateMTProtoProxy(&config); err != nil {
		jsonMsg(c, "Failed to create MTProto Proxy", err)
		return
	}
	jsonMsg(c, "MTProto Proxy created successfully", nil)
}

// getMTProtoProxy godoc
// @Summary Get an MTProto Proxy by ID
// @Description Retrieves a single MTProto Proxy configuration by its ID.
// @Tags MTProto
// @Produce json
// @Param id path int true "Proxy ID"
// @Success 200 {object} model.MTProtoProxyConfig
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /mtproto/{id} [get]
func (a *MTProtoAPI) getMTProtoProxy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid MTProto Proxy ID", err)
		return
	}
	config, err := a.mtprotoService.GetMTProtoProxy(uint(id))
	if err != nil {
		jsonMsg(c, "Failed to get MTProto Proxy", err)
		return
	}
	jsonObj(c, config, nil)
}

// updateMTProtoProxy godoc
// @Summary Update an MTProto Proxy
// @Description Updates an existing MTProto Proxy configuration.
// @Tags MTProto
// @Accept json
// @Produce json
// @Param id path int true "Proxy ID"
// @Param config body model.MTProtoProxyConfig true "MTProto Proxy Configuration"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /mtproto/{id} [put]
func (a *MTProtoAPI) updateMTProtoProxy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid MTProto Proxy ID", err)
		return
	}
	var config model.MTProtoProxyConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "Invalid MTProto Proxy config", err)
		return
	}
	config.ID = uint(id)
	if err := a.mtprotoService.UpdateMTProtoProxy(&config); err != nil {
		jsonMsg(c, "Failed to update MTProto Proxy", err)
		return
	}
	jsonMsg(c, "MTProto Proxy updated successfully", nil)
}

// deleteMTProtoProxy godoc
// @Summary Delete an MTProto Proxy
// @Description Deletes an MTProto Proxy configuration.
// @Tags MTProto
// @Produce json
// @Param id path int true "Proxy ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /mtproto/{id} [delete]
func (a *MTProtoAPI) deleteMTProtoProxy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid MTProto Proxy ID", err)
		return
	}
	if err := a.mtprotoService.DeleteMTProtoProxy(uint(id)); err != nil {
		jsonMsg(c, "Failed to delete MTProto Proxy", err)
		return
	}
	jsonMsg(c, "MTProto Proxy deleted successfully", nil)
}

// startMTProtoProxy godoc
// @Summary Start an MTProto Proxy
// @Description Starts a configured MTProto Proxy instance.
// @Tags MTProto
// @Produce json
// @Param id path int true "Proxy ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /mtproto/{id}/start [post]
func (a *MTProtoAPI) startMTProtoProxy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid MTProto Proxy ID", err)
		return
	}
	config, err := a.mtprotoService.GetMTProtoProxy(uint(id))
	if err != nil {
		jsonMsg(c, "Failed to get MTProto Proxy", err)
		return
	}
	if err := a.mtprotoService.StartMTProtoProxy(config); err != nil {
		jsonMsg(c, "Failed to start MTProto Proxy", err)
		return
	}
	jsonMsg(c, "MTProto Proxy started successfully", nil)
}

// stopMTProtoProxy godoc
// @Summary Stop an MTProto Proxy
// @Description Stops a running MTProto Proxy instance.
// @Tags MTProto
// @Produce json
// @Param id path int true "Proxy ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /mtproto/{id}/stop [post]
func (a *MTProtoAPI) stopMTProtoProxy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid MTProto Proxy ID", err)
		return
	}
	if err := a.mtprotoService.StopMTProtoProxy(uint(id)); err != nil {
		jsonMsg(c, "Failed to stop MTProto Proxy", err)
		return
	}
	jsonMsg(c, "MTProto Proxy stopped successfully", nil)
}

// generateSecret godoc
// @Summary Generate MTProto Secret
// @Description Generates a random 32-byte hexadecimal string suitable for an MTProto secret.
// @Tags MTProto
// @Produce json
// @Success 200 {object} object{secret=string}
// @Failure 500 {object} object{message=string}
// @Router /mtproto/generate-secret [get]
func (a *MTProtoAPI) generateSecret(c *gin.Context) {
	secret, err := service.GenerateMTProtoSecret()
	if err != nil {
		jsonMsg(c, "Failed to generate secret", err)
		return
	}
	jsonObj(c, gin.H{"secret": secret}, nil)
}
