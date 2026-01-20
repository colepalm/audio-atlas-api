package handlers

import (
	"github.com/gin-gonic/gin"

	"audio-atlas-api/database"
	"audio-atlas-api/models"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Check(c *gin.Context) {
	// Test database connection by counting artists
	var count int64
	err := database.DB.Model(&models.Artist{}).Count(&count).Error

	if err != nil {
		c.JSON(500, gin.H{
			"status":   "error",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":       "ok",
		"database":     "connected",
		"artist_count": count,
	})
}
