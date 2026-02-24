package handlers

import (
	"audio-atlas-api/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
)

type ConcertHandler struct {
	DB *gorm.DB
}

func NewConcertHandler(db *gorm.DB) *ConcertHandler {
	return &ConcertHandler{DB: db}
}

func (h *ConcertHandler) GetNearby(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Get top artists for user
	var userArtists []models.UserArtist
	h.DB.
		Where("user_id = ?", userID).
		Order("play_count DESC").
		Limit(100).
		Find(&userArtists)

	if len(userArtists) == 0 {
		c.JSON(http.StatusOK, gin.H{"events": []interface{}{}})
		return
	}

	var artistIDs []uuid.UUID
	for _, ua := range userArtists {
		artistIDs = append(artistIDs, ua.ArtistID)
	}

	type Result struct {
		models.Event
		PlayCount int
	}

	var results []Result

	h.DB.
		Table("events").
		Select("events.*, user_artists.play_count").
		Joins("JOIN event_artists ON events.id = event_artists.event_id").
		Joins("JOIN user_artists ON event_artists.artist_id = user_artists.artist_id").
		Where("user_artists.user_id = ?", userID).
		Where("events.city = ?", user.Location).
		Order("user_artists.play_count DESC, events.date ASC").
		Scan(&results)

	c.JSON(http.StatusOK, results)
}

func (h *ConcertHandler) GetByID(c *gin.Context) {
	eventID := c.Param("id")

	var event models.Event
	if err := h.DB.First(&event, "id = ?", eventID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	var artists []models.Artist

	h.DB.
		Table("artists").
		Joins("JOIN event_artists ON artists.id = event_artists.artist_id").
		Where("event_artists.event_id = ?", eventID).
		Find(&artists)

	c.JSON(http.StatusOK, gin.H{
		"event":   event,
		"artists": artists,
	})
}
