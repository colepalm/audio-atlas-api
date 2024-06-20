package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"audio-atlas-api/handlers"
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

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:9000"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	authHandler := handlers.NewAuthHandler(config.SpotifyClientId, config.SpotifyClientSecret, "state-string-here")

	router.GET("/", authHandler.RedirectToSpotify)
	router.GET("/callback", authHandler.HandleSpotifyCallback)

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
