package main

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"audio-atlas-api/handlers"
)

var (
	oauthConf = &oauth2.Config{
		RedirectURL:  "http://localhost:8080/callback",
		ClientID:     "your-client-id",     // Replace with your actual client ID
		ClientSecret: "your-client-secret", // Replace with your actual client secret
		Scopes:       []string{"user-read-private", "user-read-email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}
	oauthStateString = generateStateString()
)

func main() {
	router := gin.Default()

	authHandler := handlers.NewAuthHandler(oauthConf, oauthStateString)

	router.GET("/", authHandler.RedirectToSpotify)
	router.GET("/callback", authHandler.HandleSpotifyCallback)

	_ = router.Run(":8080")
}

// generateStateString generates a secure random state string for OAuth CSRF protection
func generateStateString() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		// TODO: this doesnt need to panic
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}
