package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"audio-atlas-api/models"
	"audio-atlas-api/utils"
)

type TasteHandler struct {
	DB *gorm.DB
}

func NewTasteHandler(db *gorm.DB) *TasteHandler {
	return &TasteHandler{DB: db}
}

// TopArtists GET /api/v1/me/top/artists
func (h *TasteHandler) TopArtists(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	timeRange := c.Query("time_range") // reserved for future use
	_ = timeRange

	type Result struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		PlayCount int       `json:"play_count"`
		Source    string    `json:"source"`
	}

	results := make([]Result, 0)

	err := h.DB.Table("user_artists").
		Select(`
			artists.id,
			artists.name,
			user_artists.play_count,
			user_artists.source
		`).
		Joins("JOIN artists ON artists.id = user_artists.artist_id").
		Where("user_artists.user_id = ?", userID).
		Order("user_artists.play_count DESC").
		Limit(10).
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.RespondList(c, results)
}

// TopTracks GET /api/v1/me/top/tracks
func (h *TasteHandler) TopTracks(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	type ArtistResult struct {
		Name string `json:"name"`
	}

	type Result struct {
		ID          uuid.UUID      `json:"id"`
		Name        string         `json:"name"`
		TotalWeight float64        `json:"total_weight"`
		Artists     []ArtistResult `json:"artists"`
	}

	type RawResult struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		TotalWeight float64   `json:"total_weight"`
	}

	rawResults := make([]RawResult, 0)

	err := h.DB.Table("user_track_events").
		Select(`
			tracks.id,
			tracks.name,
			SUM(user_track_events.weight) as total_weight
		`).
		Joins("JOIN tracks ON tracks.id = user_track_events.track_id").
		Where("user_track_events.user_id = ?", userID).
		Group("tracks.id, tracks.name").
		Order("total_weight DESC").
		Limit(10).
		Scan(&rawResults).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := make([]Result, 0)
	for _, r := range rawResults {
		artists := make([]ArtistResult, 0)
		h.DB.Table("artists").
			Select("artists.name").
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Where("track_artists.track_id = ?", r.ID).
			Scan(&artists)

		results = append(results, Result{
			ID:          r.ID,
			Name:        r.Name,
			TotalWeight: r.TotalWeight,
			Artists:     artists,
		})
	}

	utils.RespondList(c, results)
}

// RecentlyPlayed GET /api/v1/me/recently-played
func (h *TasteHandler) RecentlyPlayed(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	type ArtistResult struct {
		Name string `json:"name"`
	}

	type Result struct {
		ID      uuid.UUID      `json:"id"`
		Name    string         `json:"name"`
		Rank    int            `json:"rank"`
		Artists []ArtistResult `json:"artists"`
	}

	results := make([]Result, 0)

	// Find the most recent snapshot
	var snapshot models.ListeningSnapshot
	err := h.DB.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&snapshot).Error

	if err != nil {
		// No snapshot yet — return empty list
		utils.RespondList(c, results)
		return
	}

	type RawResult struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
		Rank int       `json:"rank"`
	}

	rawResults := make([]RawResult, 0)

	h.DB.Table("snapshot_tracks").
		Select("tracks.id, tracks.name, snapshot_tracks.rank").
		Joins("JOIN tracks ON tracks.id = snapshot_tracks.track_id").
		Where("snapshot_tracks.snapshot_id = ?", snapshot.ID).
		Order("snapshot_tracks.rank ASC").
		Scan(&rawResults)

	for _, r := range rawResults {
		artists := make([]ArtistResult, 0)
		h.DB.Table("artists").
			Select("artists.name").
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Where("track_artists.track_id = ?", r.ID).
			Scan(&artists)

		results = append(results, Result{
			ID:      r.ID,
			Name:    r.Name,
			Rank:    r.Rank,
			Artists: artists,
		})
	}

	utils.RespondList(c, results)
}

// NowPlaying GET /api/v1/me/player/current
// No real-time player without a live Spotify connection
func (h *TasteHandler) NowPlaying(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Recommendations GET /api/v1/me/recommendations
func (h *TasteHandler) Recommendations(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	type ArtistResult struct {
		Name string `json:"name"`
	}

	type Result struct {
		ID      uuid.UUID      `json:"id"`
		Name    string         `json:"name"`
		Artists []ArtistResult `json:"artists"`
	}

	results := make([]Result, 0)

	// Get artist IDs the user already knows
	knownArtistIDs := make([]uuid.UUID, 0)
	h.DB.Table("user_artists").
		Select("artist_id").
		Where("user_id = ?", userID).
		Scan(&knownArtistIDs)

	// Get top genres from those artists
	topGenres := make([]string, 0)
	if len(knownArtistIDs) > 0 {
		h.DB.Table("artist_genres").
			Select("genre").
			Where("artist_id IN ?", knownArtistIDs).
			Group("genre").
			Order("COUNT(*) DESC").
			Limit(3).
			Pluck("genre", &topGenres)
	}

	if len(topGenres) == 0 {
		utils.RespondList(c, results)
		return
	}

	type RawResult struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	}

	rawResults := make([]RawResult, 0)

	query := h.DB.Table("tracks").
		Select("DISTINCT tracks.id, tracks.name").
		Joins("JOIN track_artists ON track_artists.track_id = tracks.id").
		Joins("JOIN artist_genres ON artist_genres.artist_id = track_artists.artist_id").
		Where("artist_genres.genre IN ?", topGenres).
		Limit(10)

	if len(knownArtistIDs) > 0 {
		query = query.Where("track_artists.artist_id NOT IN ?", knownArtistIDs)
	}

	query.Scan(&rawResults)

	for _, r := range rawResults {
		artists := make([]ArtistResult, 0)
		h.DB.Table("artists").
			Select("artists.name").
			Joins("JOIN track_artists ON track_artists.artist_id = artists.id").
			Where("track_artists.track_id = ?", r.ID).
			Scan(&artists)

		results = append(results, Result{
			ID:      r.ID,
			Name:    r.Name,
			Artists: artists,
		})
	}

	utils.RespondList(c, results)
}

// TopGenres GET /api/v1/me/top/genres
func (h *TasteHandler) TopGenres(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	type Result struct {
		Genre string `json:"genre"`
		Count int    `json:"count"`
	}

	results := make([]Result, 0)

	err := h.DB.Table("artist_genres").
		Select("artist_genres.genre, COUNT(*) as count").
		Joins("JOIN user_artists ON user_artists.artist_id = artist_genres.artist_id").
		Where("user_artists.user_id = ?", userID).
		Group("artist_genres.genre").
		Order("count DESC").
		Limit(10).
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.RespondList(c, results)
}

// NewReleases GET /api/v1/me/new-releases
// Returns upcoming events for the user's top artists
func (h *TasteHandler) NewReleases(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	type Result struct {
		models.Event
		ArtistName string `json:"artist_name"`
	}

	results := make([]Result, 0)

	artistIDs := make([]uuid.UUID, 0)
	h.DB.Table("user_artists").
		Select("artist_id").
		Where("user_id = ?", userID).
		Order("play_count DESC").
		Limit(10).
		Pluck("artist_id", &artistIDs)

	if len(artistIDs) > 0 {
		h.DB.Table("events").
			Select("events.*, artists.name as artist_name").
			Joins("JOIN event_artists ON event_artists.event_id = events.id").
			Joins("JOIN artists ON artists.id = event_artists.artist_id").
			Where("event_artists.artist_id IN ?", artistIDs).
			Order("events.date ASC").
			Limit(10).
			Scan(&results)
	}

	utils.RespondList(c, results)
}
