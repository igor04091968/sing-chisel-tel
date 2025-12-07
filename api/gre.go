package api

import (
	"strconv"

	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/igor04091968/sing-chisel-tel/service"
	"github.com/gin-gonic/gin"
)

// GreAPI handles API requests for GRE tunnels.
type GreAPI struct {
	greService *service.GreService
}

// NewGreAPI creates a new instance of GreAPI.
func NewGreAPI() *GreAPI {
	return &GreAPI{
		greService: service.NewGreService(),
	}
}

// RegisterRoutes registers the API routes for GRE tunnels.
func (a *GreAPI) RegisterRoutes(router *gin.RouterGroup) {
	greGroup := router.Group("/gre")
	greGroup.GET("", a.getGreTunnels)
	greGroup.POST("", a.createGreTunnel)
	greGroup.DELETE("/:id", a.deleteGreTunnel)
}

// getGreTunnels godoc
// @Summary Get all GRE tunnels
// @Description Retrieves a list of all configured GRE tunnels.
// @Tags GRE
// @Produce json
// @Success 200 {array} model.GreTunnel
// @Failure 500 {object} object{message=string}
// @Router /gre [get]
func (a *GreAPI) getGreTunnels(c *gin.Context) {
	configs, err := a.greService.GetAllGreTunnels()
	if err != nil {
		jsonMsg(c, "Failed to get GRE tunnels", err)
		return
	}
	jsonObj(c, configs, nil)
}

// createGreTunnel godoc
// @Summary Create a GRE tunnel
// @Description Creates a new GRE tunnel interface based on the provided configuration. Requires root privileges.
// @Tags GRE
// @Accept json
// @Produce json
// @Param config body model.GreTunnel true "GRE Tunnel Configuration"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /gre [post]
func (a *GreAPI) createGreTunnel(c *gin.Context) {
	var config model.GreTunnel
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "Invalid GRE tunnel config", err)
		return
	}

	// NOTE: This operation requires root privileges.
	// The application must be run as root for this to succeed.
	if err := a.greService.CreateGreTunnel(&config); err != nil {
		jsonMsg(c, "Failed to create GRE tunnel", err)
		return
	}

	jsonMsg(c, "GRE tunnel created successfully", nil)
}

// deleteGreTunnel godoc
// @Summary Delete a GRE tunnel
// @Description Deletes a GRE tunnel interface and its configuration. Requires root privileges.
// @Tags GRE
// @Produce json
// @Param id path int true "Tunnel ID"
// @Success 200 {object} object{message=string}
// @Failure 400 {object} object{message=string}
// @Failure 500 {object} object{message=string}
// @Router /gre/{id} [delete]
func (a *GreAPI) deleteGreTunnel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid GRE tunnel ID", err)
		return
	}

	// NOTE: This operation requires root privileges.
	// The application must be run as root for this to succeed.
	if err := a.greService.DeleteGreTunnel(uint(id)); err != nil {
		jsonMsg(c, "Failed to delete GRE tunnel", err)
		return
	}

	jsonMsg(c, "GRE tunnel deleted successfully", nil)
}
