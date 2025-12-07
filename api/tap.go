package api

import (
	"strconv"

	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/igor04091968/sing-chisel-tel/service"
	"github.com/gin-gonic/gin"
)

// TapAPI handles API requests for TAP tunnels.
type TapAPI struct {
	tapService *service.TapService
}

// NewTapAPI creates a new instance of TapAPI.
func NewTapAPI() *TapAPI {
	return &TapAPI{
		tapService: service.NewTapService(),
	}
}

// RegisterRoutes registers the API routes for TAP tunnels.
func (a *TapAPI) RegisterRoutes(router *gin.RouterGroup) {
	tapGroup := router.Group("/tap")
	tapGroup.GET("", a.getTapTunnels)
	tapGroup.POST("", a.createTapTunnel)
	tapGroup.DELETE("/:id", a.deleteTapTunnel)
}

// getTapTunnels godoc
// @Summary Get all TAP tunnels
// @Description Retrieves a list of all configured TAP tunnels.
// @Tags TAP
// @Produce json
// @Success 200 {array} model.TapTunnel
// @Failure 500 {object} object{message=string}
// @Router /tap [get]
func (a *TapAPI) getTapTunnels(c *gin.Context) {
	configs, err := a.tapService.GetAllTapTunnels()
	if err != nil {
		jsonMsg(c, "Failed to get TAP tunnels", err)
		return
	}
	jsonObj(c, configs, nil)
}

// createTapTunnel godoc
// @Summary Create a TAP tunnel
// @Description Creates a new TAP interface based on the provided configuration. Requires root privileges for full configuration.
// @Tags TAP
// @Accept json
// @Produce json
// @Param config body model.TapTunnel true "TAP Tunnel Configuration"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /tap [post]
func (a *TapAPI) createTapTunnel(c *gin.Context) {
	var config model.TapTunnel
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "Invalid TAP tunnel config", err)
		return
	}

	// NOTE: This operation requires root privileges for full configuration.
	// The application must be run as root for this to succeed.
	if err := a.tapService.CreateTapTunnel(&config); err != nil {
		jsonMsg(c, "Failed to create TAP tunnel", err)
		return
	}

	jsonMsg(c, "TAP tunnel created successfully", nil)
}

// deleteTapTunnel godoc
// @Summary Delete a TAP tunnel
// @Description Deletes a TAP interface and its configuration. Requires root privileges.
// @Tags TAP
// @Produce json
// @Param id path int true "Tunnel ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /tap/{id} [delete]
func (a *TapAPI) deleteTapTunnel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid TAP tunnel ID", err)
		return
	}

	// NOTE: This operation requires root privileges.
	// The application must be run as root for this to succeed.
	if err := a.tapService.DeleteTapTunnel(uint(id)); err != nil {
		jsonMsg(c, "Failed to delete TAP tunnel", err)
		return
	}

	jsonMsg(c, "TAP tunnel deleted successfully", nil)
}
