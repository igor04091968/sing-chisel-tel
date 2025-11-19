package api

import (
	"net/http"
	"strconv"

	"github.com/alireza0/s-ui/database/model"
	// "github.com/alireza0/s-ui/logger" // Removed unused import
	"github.com/alireza0/s-ui/service"
	"github.com/alireza0/s-ui/util/common"
	"github.com/gin-gonic/gin"
)

// Udp2rawAPI handles API requests for Udp2raw configurations.
type Udp2rawAPI struct {
	udp2rawService *service.Udp2rawService
}

// NewUdp2rawAPI creates a new Udp2rawAPI instance.
func NewUdp2rawAPI(s *service.Udp2rawService) *Udp2rawAPI {
	return &Udp2rawAPI{
		udp2rawService: s,
	}
}

// RegisterRoutes registers the API routes for Udp2raw configurations.
func (a *Udp2rawAPI) RegisterRoutes(g *gin.RouterGroup) {
	udp2rawGroup := g.Group("/udp2raw")
	{
		udp2rawGroup.GET("", a.getUdp2rawConfigs)
		udp2rawGroup.POST("", a.createUdp2rawConfig)
		udp2rawGroup.GET("/:id", a.getUdp2rawConfig)
		udp2rawGroup.PUT("/:id", a.updateUdp2rawConfig)
		udp2rawGroup.DELETE("/:id", a.deleteUdp2rawConfig)
		udp2rawGroup.POST("/:id/start", a.startUdp2raw)
		udp2rawGroup.POST("/:id/stop", a.stopUdp2raw)
	}
}

// getUdp2rawConfigs handles GET /api/v2/udp2raw requests.
func (a *Udp2rawAPI) getUdp2rawConfigs(c *gin.Context) {
	configs, err := a.udp2rawService.GetAll()
	if err != nil {
		jsonMsg(c, "failed to get udp2raw configs", common.NewError("failed to get udp2raw configs: %v", err))
		return
	}
	c.JSON(http.StatusOK, configs)
}

// createUdp2rawConfig handles POST /api/v2/udp2raw requests.
func (a *Udp2rawAPI) createUdp2rawConfig(c *gin.Context) {
	var config model.Udp2rawConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "invalid request body", common.NewError("invalid request body: %v", err))
		return
	}

	if err := a.udp2rawService.Save(&config); err != nil {
		jsonMsg(c, "failed to create udp2raw config", common.NewError("failed to create udp2raw config: %v", err))
		return
	}
	jsonMsg(c, "udp2raw config created successfully", nil)
}

// getUdp2rawConfig handles GET /api/v2/udp2raw/:id requests.
func (a *Udp2rawAPI) getUdp2rawConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "invalid ID", common.NewError("invalid ID: %v", err))
		return
	}

	config, err := a.udp2rawService.GetByID(uint(id)) // Get by ID
	if err != nil {
		jsonMsg(c, "udp2raw config not found", common.NewError("udp2raw config not found: %v", err))
		return
	}
	c.JSON(http.StatusOK, config)
}

// updateUdp2rawConfig handles PUT /api/v2/udp2raw/:id requests.
func (a *Udp2rawAPI) updateUdp2rawConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "invalid ID", common.NewError("invalid ID: %v", err))
		return
	}

	var config model.Udp2rawConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "invalid request body", common.NewError("invalid request body: %v", err))
		return
	}
	config.ID = uint(id) // Ensure ID from URL is used

	if err := a.udp2rawService.Save(&config); err != nil {
		jsonMsg(c, "failed to update udp2raw config", common.NewError("failed to update udp2raw config: %v", err))
		return
	}
	jsonMsg(c, "udp2raw config updated successfully", nil)
}

// deleteUdp2rawConfig handles DELETE /api/v2/udp2raw/:id requests.
func (a *Udp2rawAPI) deleteUdp2rawConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "invalid ID", common.NewError("invalid ID: %v", err))
		return
	}

	// Get config by ID first to ensure we have the name for deletion
	config, err := a.udp2rawService.GetByID(uint(id))
	if err != nil {
		jsonMsg(c, "udp2raw config not found", common.NewError("udp2raw config not found: %v", err))
		return
	}

	if err := a.udp2rawService.Delete(config.Name); err != nil {
		jsonMsg(c, "failed to delete udp2raw config", common.NewError("failed to delete udp2raw config: %v", err))
		return
	}
	jsonMsg(c, "udp2raw config deleted successfully", nil)
}

// startUdp2raw handles POST /api/v2/udp2raw/:id/start requests.
func (a *Udp2rawAPI) startUdp2raw(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "invalid ID", common.NewError("invalid ID: %v", err))
		return
	}

	config, err := a.udp2rawService.GetByID(uint(id))
	if err != nil {
		jsonMsg(c, "udp2raw config not found", common.NewError("udp2raw config not found: %v", err))
		return
	}

	if err := a.udp2rawService.Start(config); err != nil {
		jsonMsg(c, "failed to start udp2raw tunnel", common.NewError("failed to start udp2raw tunnel: %v", err))
		return
	}
	jsonMsg(c, "udp2raw tunnel started successfully", nil)
}

// stopUdp2raw handles POST /api/v2/udp2raw/:id/stop requests.
func (a *Udp2rawAPI) stopUdp2raw(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "invalid ID", common.NewError("invalid ID: %v", err))
		return
	}

	config, err := a.udp2rawService.GetByID(uint(id))
	if err != nil {
		jsonMsg(c, "udp2raw config not found", common.NewError("udp2raw config not found: %v", err))
		return
	}

	if err := a.udp2rawService.Stop(config); err != nil {
		jsonMsg(c, "failed to stop udp2raw tunnel", common.NewError("failed to stop udp2raw tunnel: %v", err))
		return
	}
	jsonMsg(c, "udp2raw tunnel stopped successfully", nil)
}
