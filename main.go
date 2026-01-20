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

	// Initialize Database
	if err := database.InitDatabase(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Setup routes
	router := routes.SetupRoutes(cfg)

	// Start server
	log.Printf("Starting server on port %s...", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
