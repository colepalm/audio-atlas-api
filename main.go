package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"audio-atlas-api/handlers"
	"audio-atlas-api/middleware"
)

func main() {
	spotifyClientID := os.Getenv("SPOTIFY_CLIENT_ID")
	spotifyClientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	stateString := os.Getenv("STATE_STRING")
	redirectURL := os.Getenv("SPOTIFY_REDIRECT_URL")

	// Validate that required environment variables are set
	if spotifyClientID == "" || spotifyClientSecret == "" || redirectURL == "" {
		fmt.Println("Error: Missing required environment variables")
		os.Exit(1)
	}

	router := gin.Default()

	router.Use(middleware.CORSMiddleware())

	authHandler := handlers.NewAuthHandler(
		spotifyClientID,
		spotifyClientSecret,
		stateString,
		redirectURL,
	)

	router.POST("/api/spotify/token", authHandler.ExchangeCodeForToken)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}
	err := router.Run(":" + port)
	if err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
}
