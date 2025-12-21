package main

import (
	"log"

	"audio-atlas-api/config"
	"audio-atlas-api/database"
	"audio-atlas-api/routes"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize Supabase
	if err := database.InitSupabase(); err != nil {
		log.Fatal("Failed to initialize Supabase:", err)
	}

	// Setup routes
	router := routes.SetupRoutes(cfg)

	// Start server
	log.Printf("Starting server on port %s...", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
