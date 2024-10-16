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
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Println("Error loading config:", err)
		os.Exit(1)
	}

	router := gin.Default()

	router.Use(middleware.CORSMiddleware())

	authHandler := handlers.NewAuthHandler(config.SpotifyClientId, config.SpotifyClientSecret, "state-string-here")

	router.POST("/api/spotify/token", authHandler.ExchangeCodeForToken)

	_ = router.Run(":8080")
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
