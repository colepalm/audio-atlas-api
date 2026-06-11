package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"audio-atlas-api/models"
	"audio-atlas-api/utils"
)

func (h *Handler) Refresh(c *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Validate the JWT signature and expiry
	token, err := jwt.Parse(body.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenInvalidClaims
		}
		return h.JWTSecret, nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token type"})
		return
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
		return
	}

	// Check it exists in DB and isn't expired
	hash := sha256.Sum256([]byte(body.RefreshToken))
	tokenHash := hex.EncodeToString(hash[:])

	var stored models.RefreshToken
	if err := h.DB.Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).
		First(&stored).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token not found or expired"})
		return
	}

	// Rotate — delete old, issue new
	h.DB.Delete(&stored)

	newAccessToken, err := utils.GenerateJWT(stored.UserID, h.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation failed"})
		return
	}

	newRefreshToken, err := utils.GenerateRefreshToken(stored.UserID, h.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation failed"})
		return
	}

	newHash := sha256.Sum256([]byte(newRefreshToken))
	h.DB.Create(&models.RefreshToken{
		UserID:    stored.UserID,
		TokenHash: hex.EncodeToString(newHash[:]),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	})

	_ = userIDStr // already validated via stored record
	c.JSON(http.StatusOK, gin.H{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
	})
}
