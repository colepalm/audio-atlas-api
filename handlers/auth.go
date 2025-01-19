package handlers

import (
	"golang.org/x/oauth2"
	"net/http"
	"time"

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
			RedirectURL:  "http://localhost:9000/spotify-callback",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes: []string{
				"user-read-private",
				"user-read-email",
				"playlist-modify-public",
				"playlist-modify-private",
			},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.spotify.com/authorize",
				TokenURL: "https://accounts.spotify.com/api/token",
			},
		},
		OAuthState: state,
	}
}

// ExchangeCodeForToken handles exchanging the authorization code for an access token
func (a *AuthHandler) ExchangeCodeForToken(c *gin.Context) {
	var requestData struct {
		Code string `json:"code"`
	}

	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	code := requestData.Code
	token, err := a.OAuthConfig.Exchange(c, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}

	// Return the tokens to the frontend
	c.JSON(http.StatusOK, gin.H{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_in":    int(token.Expiry.Sub(time.Now()).Seconds()),
	})
}

func (a *AuthHandler) RefreshToken(c *gin.Context) {
	var requestData struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	tokenSource := a.OAuthConfig.TokenSource(c, &oauth2.Token{
		RefreshToken: requestData.RefreshToken,
	})
	newToken, err := tokenSource.Token()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": newToken.AccessToken,
		"expires_in":   int(newToken.Expiry.Sub(time.Now()).Seconds()),
	})
}
