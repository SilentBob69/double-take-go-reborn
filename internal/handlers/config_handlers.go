package handlers

import (
	"net/http"

	"double-take-go-reborn/internal/config"

	"github.com/gin-gonic/gin"
)

// ConfigHandler holds the application configuration.
// This could be expanded later to hold DB connections or other shared resources if needed.
type ConfigHandler struct {
	Cfg *config.Config
}

// NewConfigHandler creates a new handler for configuration endpoints.
func NewConfigHandler(cfg *config.Config) *ConfigHandler {
	return &ConfigHandler{Cfg: cfg}
}

// GetConfig returns the current application configuration (excluding sensitive data if necessary later).
// @Summary Get application configuration
// @Description Retrieves the currently loaded configuration.
// @Tags Config
// @Produce json
// @Success 200 {object} config.Config
// @Router /api/config [get]
func (h *ConfigHandler) GetConfig(c *gin.Context) {
	// TODO: Consider redacting sensitive parts of the config if any are added
	c.JSON(http.StatusOK, h.Cfg)
}
