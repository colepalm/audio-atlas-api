package handlers

import (
	"audio-atlas-api/database"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Check(c *gin.Context) {
	// Test database connection
	var result []map[string]interface{}
	err := database.Client.DB.From("artists").Select("*").Limit(1).Execute(&result)

	if err != nil {
		c.JSON(500, gin.H{
			"status":   "error",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"status":   "ok",
		"database": "connected",
	})
}
