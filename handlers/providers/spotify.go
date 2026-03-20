package providers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"audio-atlas-api/models"
)

type SpotifyHandler struct {
	OAuthConfig *oauth2.Config
	DB          *gorm.DB
}

func NewSpotifyHandler(cfg *oauth2.Config, db *gorm.DB) *SpotifyHandler {
	return &SpotifyHandler{
		OAuthConfig: cfg,
		DB:          db,
	}
}

func (h *SpotifyHandler) Connect(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	state := userID.String()

	url := h.OAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *SpotifyHandler) Callback(c *gin.Context) {
	state := c.Query("state")
	userID, _ := uuid.Parse(state)

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code"})
		return
	}

	token, err := h.OAuthConfig.Exchange(c, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth exchange failed"})
		return
	}

	client := h.OAuthConfig.Client(c, token)

	resp, err := client.Get("https://api.spotify.com/v1/me")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Spotify profile"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Spotify API error"})
		return
	}

	var profile struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Spotify response"})
		return
	}

	account := models.ProviderAccount{
		UserID:         userID,
		Provider:       "spotify",
		ProviderUserID: profile.ID,
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		Expiry:         token.Expiry,
	}

	err = h.DB.
		Where("provider = ? AND provider_user_id = ?", "spotify", profile.ID).
		Assign(account).
		FirstOrCreate(&account).Error

	go func() {
		// TODO:
		//  services.SyncSpotifyUser(userID, token.AccessToken)
	}()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save provider account"})
		return
	}
}
