package config

import (
	"fmt"
	"os"
)

type Config struct {
	// Spotify
	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyRedirectURL  string
	StateString         string

	// Database
	DatabaseURL string

	// Server
	Port string

	JWTSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		SpotifyClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		SpotifyClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		SpotifyRedirectURL:  os.Getenv("SPOTIFY_REDIRECT_URL"),
		StateString:         os.Getenv("STATE_STRING"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Port:                os.Getenv("PORT"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
	}

	// Set defaults
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	// Validate required fields
	if cfg.SpotifyClientID == "" {
		return nil, fmt.Errorf("SPOTIFY_CLIENT_ID is required")
	}
	if cfg.SpotifyClientSecret == "" {
		return nil, fmt.Errorf("SPOTIFY_CLIENT_SECRET is required")
	}
	if cfg.SpotifyRedirectURL == "" {
		return nil, fmt.Errorf("SPOTIFY_REDIRECT_URL is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}
