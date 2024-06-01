package handlers

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"net/http"
)

// AuthHandler holds necessary OAuth configuration and state
type AuthHandler struct {
	OAuthConfig *oauth2.Config
	OAuthState  string
}

// NewAuthHandler creates a new AuthHandler with provided configuration
func NewAuthHandler(config *oauth2.Config, state string) *AuthHandler {
	return &AuthHandler{
		OAuthConfig: config,
		OAuthState:  state,
	}
}

// RedirectToSpotify redirects the user to Spotify's authorization page
func (a *AuthHandler) RedirectToSpotify(c *gin.Context) {
	state := a.OAuthState // You might want to generate a new state for each session
	url := a.OAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// HandleSpotifyCallback handles the OAuth 2.0 callback from Spotify
func (a *AuthHandler) HandleSpotifyCallback(c *gin.Context) {
	state, code := c.Query("state"), c.Query("code")
	if state != a.OAuthState {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid state parameter"})
		return
	}
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Code not found"})
		return
	}
	token, err := a.OAuthConfig.Exchange(c, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": token.AccessToken})
}
