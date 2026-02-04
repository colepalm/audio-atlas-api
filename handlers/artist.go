package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"audio-atlas-api/models"
)

type ArtistHandler struct {
	db *gorm.DB
}

func NewArtistHandler(db *gorm.DB) *ArtistHandler {
	return &ArtistHandler{db: db}
}

type SyncArtistsRequest struct {
	Source    string              `json:"source" binding:"required"`
	TimeRange string              `json:"time_range" binding:"required"`
	Artists   []SyncArtistPayload `json:"artists" binding:"required"`
}

type SyncArtistPayload struct {
	Name      string `json:"name" binding:"required"`
	PlayCount int    `json:"play_count"`
}

func (h *ArtistHandler) Sync(c *gin.Context) {
	var req SyncArtistsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: replace with auth middleware context
	userIDRaw, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDRaw.(uuid.UUID)

	now := time.Now()

	for _, a := range req.Artists {
		normalized := normalizeArtistName(a.Name)

		var artist models.Artist
		err := h.db.
			Where("normalized_name = ?", normalized).
			First(&artist).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				artist = models.Artist{
					ID:             uuid.New(),
					Name:           a.Name,
					NormalizedName: normalized,
					CreatedAt:      now,
				}
				if err := h.db.Create(&artist).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		var ua models.UserArtist
		err = h.db.
			Where("user_id = ? AND artist_id = ?", userID, artist.ID).
			First(&ua).Error

		if err == gorm.ErrRecordNotFound {
			ua = models.UserArtist{
				UserID:     userID,
				ArtistID:   artist.ID,
				Source:     req.Source,
				PlayCount:  a.PlayCount,
				LastSynced: now,
			}
			h.db.Create(&ua)
		} else {
			ua.PlayCount = a.PlayCount
			ua.LastSynced = now
			h.db.Save(&ua)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "synced",
		"count":  len(req.Artists),
	})
}

func (h *ArtistHandler) GetUserArtists(c *gin.Context) {
	userIDRaw, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDRaw.(uuid.UUID)

	type Result struct {
		ArtistID  uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		PlayCount int       `json:"play_count"`
		Source    string    `json:"source"`
	}

	var results []Result

	err := h.db.Table("user_artists").
		Select(`
			artists.id as artist_id,
			artists.name,
			user_artists.play_count,
			user_artists.source
		`).
		Joins("JOIN artists ON artists.id = user_artists.artist_id").
		Where("user_artists.user_id = ?", userID).
		Order("user_artists.play_count DESC").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"artists": results})
}

func normalizeArtistName(name string) string {
	n := strings.ToLower(name)
	n = strings.TrimSpace(n)
	n = regexp.MustCompile(`[^a-z0-9\s]`).ReplaceAllString(n, "")
	n = regexp.MustCompile(`\s+`).ReplaceAllString(n, " ")
	return n
}
