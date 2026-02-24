package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"time"

	"audio-atlas-api/models"
)

type PlaylistHandler struct {
	DB *gorm.DB
}

func NewPlaylistHandler(db *gorm.DB) *PlaylistHandler {
	return &PlaylistHandler{DB: db}
}

func (h *PlaylistHandler) Create(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var body struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	playlist := models.Playlist{
		UserID: userID,
		Name:   body.Name,
	}

	if err := h.DB.Create(&playlist).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create playlist"})
		return
	}

	c.JSON(http.StatusCreated, playlist)
}

func (h *PlaylistHandler) List(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var playlists []models.Playlist

	if err := h.DB.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&playlists).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch playlists"})
		return
	}

	c.JSON(http.StatusOK, playlists)
}

func (h *PlaylistHandler) Get(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	playlistID := c.Param("id")

	var playlist models.Playlist

	if err := h.DB.
		Where("id = ? AND user_id = ?", playlistID, userID).
		First(&playlist).Error; err != nil {

		c.JSON(http.StatusNotFound, gin.H{"error": "Playlist not found"})
		return
	}

	var tracks []models.Track

	h.DB.
		Joins("JOIN playlist_tracks ON playlist_tracks.track_id = tracks.id").
		Where("playlist_tracks.playlist_id = ?", playlistID).
		Order("playlist_tracks.position ASC").
		Find(&tracks)

	c.JSON(http.StatusOK, gin.H{
		"playlist": playlist,
		"tracks":   tracks,
	})
}

func (h *PlaylistHandler) AddTracks(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	playlistID := c.Param("id")

	var body struct {
		Tracks []struct {
			Name   string `json:"name" binding:"required"`
			Artist string `json:"artist" binding:"required"`
		} `json:"tracks" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tx := h.DB.Begin()

	for _, t := range body.Tracks {

		// Upsert artist
		var artist models.Artist
		tx.Where("normalized_name = ?", normalize(t.Artist)).
			FirstOrCreate(&artist, models.Artist{
				Name:           t.Artist,
				NormalizedName: normalize(t.Artist),
			})

		// Upsert track
		var track models.Track
		tx.Where("name = ? AND artist_id = ?", t.Name, artist.ID).
			FirstOrCreate(&track, models.Track{
				Name:     t.Name,
				ArtistID: artist.ID,
			})

		// Add to playlist
		tx.Create(&models.PlaylistTrack{
			PlaylistID: uuid.MustParse(playlistID),
			TrackID:    track.ID,
			AddedAt:    time.Now(),
		})

		// Emit taste signal
		tx.Create(&models.UserTrackEvent{
			UserID:  userID,
			TrackID: track.ID,
			Type:    "playlist_add",
			Source:  "manual",
			Weight:  1,
		})
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add tracks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "tracks added"})
}

func (h *PlaylistHandler) Delete(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	playlistID := c.Param("id")

	if err := h.DB.
		Where("id = ? AND user_id = ?", playlistID, userID).
		Delete(&models.Playlist{}).Error; err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete playlist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func normalize(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}
