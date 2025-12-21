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

	// Supabase
	SupabaseURL        string
	SupabaseServiceKey string

	// Server
	Port string
}

func Load() (*Config, error) {
	cfg := &Config{
		SpotifyClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		SpotifyClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		SpotifyRedirectURL:  os.Getenv("SPOTIFY_REDIRECT_URL"),
		StateString:         os.Getenv("STATE_STRING"),
		SupabaseURL:         os.Getenv("SUPABASE_URL"),
		SupabaseServiceKey:  os.Getenv("SUPABASE_SERVICE_KEY"),
		Port:                os.Getenv("PORT"),
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
	if cfg.SupabaseURL == "" {
		return nil, fmt.Errorf("SUPABASE_URL is required")
	}
	if cfg.SupabaseServiceKey == "" {
		return nil, fmt.Errorf("SUPABASE_SERVICE_KEY is required")
	}

	return cfg, nil
}
