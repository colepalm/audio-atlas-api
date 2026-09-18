package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"audio-atlas-api/models"
	"audio-atlas-api/utils"
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
		Source: "manual",
	}

	if err := h.DB.Create(&playlist).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create playlist"})
		return
	}

	c.JSON(http.StatusCreated, playlist)
}

func (h *PlaylistHandler) List(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	type Result struct {
		ID         uuid.UUID `json:"id"`
		Name       string    `json:"name"`
		Source     string    `json:"source"`
		TrackCount int64     `json:"trackCount"`
		CreatedAt  time.Time `json:"createdAt"`
	}

	results := make([]Result, 0)

	if err := h.DB.Table("playlists").
		Select("playlists.id, playlists.name, playlists.source, playlists.created_at, COUNT(playlist_tracks.track_id) as track_count").
		Joins("LEFT JOIN playlist_tracks ON playlist_tracks.playlist_id = playlists.id").
		Where("playlists.user_id = ?", userID).
		Group("playlists.id").
		Order("playlists.created_at DESC").
		Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch playlists"})
		return
	}

	utils.RespondList(c, results)
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

	type ArtistResult struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}

	type TrackResult struct {
		ID      uuid.UUID      `json:"id"`
		Name    string         `json:"name"`
		AddedAt time.Time      `json:"addedAt"`
		Artists []ArtistResult `json:"artists"`
	}

	type RawTrack struct {
		ID      uuid.UUID `json:"id"`
		Name    string    `json:"name"`
		AddedAt time.Time `json:"addedAt"`
	}

	rawTracks := make([]RawTrack, 0)

	h.DB.Table("tracks").
		Select("tracks.id, tracks.name, playlist_tracks.added_at").
		Joins("JOIN playlist_tracks ON playlist_tracks.track_id = tracks.id").
		Where("playlist_tracks.playlist_id = ?", playlistID).
		Order("playlist_tracks.added_at ASC").
		Scan(&rawTracks)

	tracks := make([]TrackResult, 0)
	for _, t := range rawTracks {
		artists := make([]ArtistResult, 0)
		h.DB.Table("artists").
			Select("artists.id, artists.name").
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Where("track_artists.track_id = ?", t.ID).
			Scan(&artists)

		tracks = append(tracks, TrackResult{
			ID:      t.ID,
			Name:    t.Name,
			AddedAt: t.AddedAt,
			Artists: artists,
		})
	}

	utils.RespondOne(c, gin.H{
		"playlist": playlist,
		"tracks":   tracks,
	})
}

func (h *PlaylistHandler) AddTracks(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	playlistID := c.Param("id")

	var body struct {
		Tracks []struct {
			Name    string   `json:"name" binding:"required"`
			Artists []string `json:"artists" binding:"required"`
		} `json:"tracks" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tx := h.DB.Begin()

	for _, t := range body.Tracks {
		var artists []models.Artist
		for _, artistName := range t.Artists {
			var artist models.Artist
			if err := tx.Where("normalized_name = ?", normalize(artistName)).
				FirstOrCreate(&artist, models.Artist{
					Name:           artistName,
					NormalizedName: normalize(artistName),
				}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert artist"})
				return
			}
			artists = append(artists, artist)
		}

		var track models.Track
		if err := tx.Where("normalized = ?", normalize(t.Name)).
			FirstOrCreate(&track, models.Track{
				Name:       t.Name,
				Normalized: normalize(t.Name),
			}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert track"})
			return
		}

		for _, artist := range artists {
			if err := tx.Where(models.TrackArtist{TrackID: track.ID, ArtistID: artist.ID}).
				FirstOrCreate(&models.TrackArtist{
					TrackID:  track.ID,
					ArtistID: artist.ID,
				}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link track to artist"})
				return
			}
		}

		if err := tx.Create(&models.PlaylistTrack{
			PlaylistID: uuid.MustParse(playlistID),
			TrackID:    track.ID,
			AddedAt:    time.Now(),
		}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add track to playlist"})
			return
		}

		if err := tx.Create(&models.UserTrackEvent{
			UserID:  userID,
			TrackID: track.ID,
			Type:    "playlist_add",
			Source:  "manual",
			Weight:  1,
		}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record taste event"})
			return
		}
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
