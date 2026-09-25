package providers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"audio-atlas-api/models"
	"audio-atlas-api/services/spotify"
)

type SpotifyHandler struct {
	OAuthConfig *oauth2.Config
	DB          *gorm.DB
	FrontendURL string
}

func NewSpotifyHandler(cfg *oauth2.Config, db *gorm.DB, frontendURL string) *SpotifyHandler {
	return &SpotifyHandler{
		OAuthConfig: cfg,
		DB:          db,
		FrontendURL: frontendURL,
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
	userID, err := uuid.Parse(state)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/?error=invalid_state")
		return
	}

	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/?error=missing_code")
		return
	}

	token, err := h.OAuthConfig.Exchange(c, code)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/?error=oauth_failed")
		return
	}

	client := h.OAuthConfig.Client(c, token)

	resp, err := client.Get("https://api.spotify.com/v1/me")
	if err != nil || resp.StatusCode != http.StatusOK {
		c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/?error=spotify_profile_failed")
		return
	}
	defer resp.Body.Close()

	var profile struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/?error=invalid_profile")
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

	if err := h.DB.
		Where("provider = ? AND provider_user_id = ?", "spotify", profile.ID).
		Assign(account).
		FirstOrCreate(&account).Error; err != nil {
		c.Redirect(http.StatusTemporaryRedirect, h.FrontendURL+"/?error=db_failed")
		return
	}

	// Run sync in background so user isn't waiting
	go func() {
		sync := spotify.NewSyncService(h.DB, userID, token.AccessToken)
		sync.Run()
	}()

	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?spotify=connected", h.FrontendURL))
}
