package routes

import (
	"audio-atlas-api/config"
	"audio-atlas-api/handlers"
	"audio-atlas-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(cfg *config.Config) *gin.Engine {
	router := gin.Default()

	// Middleware
	router.Use(middleware.CORSMiddleware())

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(
		cfg.SpotifyClientID,
		cfg.SpotifyClientSecret,
		cfg.StateString,
		cfg.SpotifyRedirectURL,
	)
	healthHandler := handlers.NewHealthHandler()

	// Routes
	api := router.Group("/api")
	{
		// Health check
		api.GET("/health", healthHandler.Check)

		// Spotify auth
		spotify := api.Group("/spotify")
		{
			spotify.POST("/token", authHandler.ExchangeCodeForToken)
		}

		// Artists
		// artists := api.Group("/artists")
		// {
		//     artists.POST("/sync", artistHandler.Sync)
		//     artists.GET("", artistHandler.GetUserArtists)
		// }
	}

	return router
}
