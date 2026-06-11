package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"audio-atlas-api/models"
)

type MeResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Location  string    `json:"location"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) Me(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID.(uuid.UUID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, MeResponse{
		ID:        user.ID,
		Email:     user.Email,
		Location:  user.Location,
		CreatedAt: user.CreatedAt,
	})
}
