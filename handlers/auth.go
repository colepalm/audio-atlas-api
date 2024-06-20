package handlers

import (
	"golang.org/x/oauth2"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthHandler holds necessary OAuth configuration and state
type AuthHandler struct {
	OAuthConfig *oauth2.Config
	OAuthState  string
}

// NewAuthHandler creates a new AuthHandler with provided configuration
func NewAuthHandler(clientID, clientSecret, state string) *AuthHandler {
	return &AuthHandler{
		OAuthConfig: &oauth2.Config{
			RedirectURL:  "http://localhost:8080/callback",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       []string{"user-read-private", "user-read-email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.spotify.com/authorize",
				TokenURL: "https://accounts.spotify.com/api/token",
			},
		},
		OAuthState: state,
	}
}

// RedirectToSpotify redirects the user to Spotify's authorization page
func (a *AuthHandler) RedirectToSpotify(c *gin.Context) {
	url := a.OAuthConfig.AuthCodeURL(a.OAuthState, oauth2.AccessTypeOffline)
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
