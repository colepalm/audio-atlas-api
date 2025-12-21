package main

import (
	"fmt"
	"log"
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

	if err := database.InitSupabase(); err != nil {
		log.Fatal("Failed to initialize Supabase:", err)
	}

	router := gin.Default()

	router.Use(middleware.CORSMiddleware())

	authHandler := handlers.NewAuthHandler(
		spotifyClientID,
		spotifyClientSecret,
		stateString,
		redirectURL,
	)

	router.GET("/api/health", func(c *gin.Context) {
		var result []map[string]interface{}
		err := database.Client.DB.From("artists").Select("*").Limit(1).Execute(&result)

		if err != nil {
			c.JSON(500, gin.H{"error": "Database connection failed", "details": err.Error()})
			return
		}

		c.JSON(200, gin.H{"status": "ok", "database": "connected"})
	})

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
