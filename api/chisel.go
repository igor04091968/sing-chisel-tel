package api

import (
	"strconv"

	"github.com/igor04091968/sing-chisel-tel/database/model"
	"github.com/gin-gonic/gin"
)

func (a *APIv2Handler) getChiselConfigs(c *gin.Context) {
	configs, err := a.ChiselService.GetAllChiselConfigs()
	if err != nil {
		jsonMsg(c, "Failed to get chisel configs", err)
		return
	}
	jsonObj(c, configs, nil)
}

func (a *APIv2Handler) createChiselConfig(c *gin.Context) {
	var config model.ChiselConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "Invalid chisel config", err)
		return
	}
	if err := a.ChiselService.CreateChiselConfig(&config); err != nil {
		jsonMsg(c, "Failed to create chisel config", err)
		return
	}
	jsonMsg(c, "Chisel config created successfully", nil)
}

func (a *APIv2Handler) getChiselConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid chisel config ID", err)
		return
	}
	config, err := a.ChiselService.GetChiselConfig(uint(id))
	if err != nil {
		jsonMsg(c, "Failed to get chisel config", err)
		return
	}
	jsonObj(c, config, nil)
}

func (a *APIv2Handler) updateChiselConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid chisel config ID", err)
		return
	}
	var config model.ChiselConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		jsonMsg(c, "Invalid chisel config", err)
		return
	}
	config.ID = uint(id)
	if err := a.ChiselService.UpdateChiselConfig(&config); err != nil {
		jsonMsg(c, "Failed to update chisel config", err)
		return
	}
	jsonMsg(c, "Chisel config updated successfully", nil)
}

func (a *APIv2Handler) deleteChiselConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid chisel config ID", err)
		return
	}
	if err := a.ChiselService.DeleteChiselConfig(uint(id)); err != nil {
		jsonMsg(c, "Failed to delete chisel config", err)
		return
	}
	jsonMsg(c, "Chisel config deleted successfully", nil)
}

func (a *APIv2Handler) startChisel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid chisel config ID", err)
		return
	}
	config, err := a.ChiselService.GetChiselConfig(uint(id))
	if err != nil {
		jsonMsg(c, "Failed to get chisel config", err)
		return
	}
	if err := a.ChiselService.StartChisel(config); err != nil {
		jsonMsg(c, "Failed to start chisel", err)
		return
	}
	jsonMsg(c, "Chisel started successfully", nil)
}

func (a *APIv2Handler) stopChisel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		jsonMsg(c, "Invalid chisel config ID", err)
		return
	}
	config, err := a.ChiselService.GetChiselConfig(uint(id))
	if err != nil {
		jsonMsg(c, "Failed to get chisel config", err)
		return
	}
	if err := a.ChiselService.StopChisel(config); err != nil {
		jsonMsg(c, "Failed to stop chisel", err)
		return
	}
	jsonMsg(c, "Chisel stopped successfully", nil)
}
