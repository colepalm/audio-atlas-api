package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"audio-atlas-api/models"
	"audio-atlas-api/services/providerauth"
	"audio-atlas-api/services/spotify"
)

// syncTimeout caps a single background sync so a stuck request can't hold the
// per-user lock forever.
const syncTimeout = 15 * time.Minute

type SpotifyHandler struct {
	OAuthConfig *oauth2.Config
	DB          *gorm.DB
	FrontendURL string

	syncing sync.Map // user ID -> struct{}; one sync per user at a time
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
	h.startSync(userID, &account)

	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?spotify=connected", h.FrontendURL))
}

// Sync POST /api/v1/providers/spotify/sync re-runs the sync for an already
// connected account. The access token is refreshed if it has expired.
func (h *SpotifyHandler) Sync(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var account models.ProviderAccount
	if err := h.DB.Where("user_id = ? AND provider = ?", userID, "spotify").First(&account).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "spotify not connected"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !h.startSync(userID, &account) {
		c.JSON(http.StatusConflict, gin.H{"error": "sync already in progress"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "sync started"})
}

// startSync runs a sync in the background and reports false if one is already
// running for this user.
func (h *SpotifyHandler) startSync(userID uuid.UUID, account *models.ProviderAccount) bool {
	if _, running := h.syncing.LoadOrStore(userID, struct{}{}); running {
		return false
	}

	go func() {
		defer h.syncing.Delete(userID)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[SpotifySync] panic for user %s: %v", userID, r)
			}
		}()

		// Not the request context: it's cancelled as soon as the response is sent.
		ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
		defer cancel()

		client := providerauth.Client(ctx, h.DB, h.OAuthConfig, account)
		spotify.NewSyncService(h.DB, userID, client).Run(ctx)
	}()
	return true
}
