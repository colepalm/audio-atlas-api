package handlers

import (
	"github.com/gin-gonic/gin"

	"audio-atlas-api/database"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Check(c *gin.Context) {
	// Test database connection
	data, _, err := database.Client.From("artists").Select("*", "", false).Limit(1, "").Execute()

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
		"data":     string(data),
	})
}
