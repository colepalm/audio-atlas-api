package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"audio-atlas-api/handlers"
	"audio-atlas-api/middleware"
)

type Config struct {
	SpotifyClientId     string `json:"spotifyClientId"`
	SpotifyClientSecret string `json:"spotifyClientSecret"`
}

func main() {
	spotifyClientID := os.Getenv("SPOTIFY_CLIENT_ID")
	spotifyClientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	stateString := os.Getenv("STATE_STRING")

	// Validate that required environment variables are set
	if spotifyClientID == "" || spotifyClientSecret == "" {
		fmt.Println("Error: Missing required environment variables")
		os.Exit(1)
	}

	router := gin.Default()

	router.Use(middleware.CORSMiddleware())

	authHandler := handlers.NewAuthHandler(spotifyClientID, spotifyClientSecret, stateString)

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

func loadConfig(path string) (*Config, error) {
	configFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}

	defer func(configFile *os.File) {
		_ = configFile.Close()
	}(configFile)

	var config Config
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}
	return &config, nil
}
