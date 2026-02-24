package routes

import (
	"audio-atlas-api/database"
	"github.com/gin-gonic/gin"

	"audio-atlas-api/config"
	"audio-atlas-api/handlers"
	"audio-atlas-api/middleware"
)

func SetupRoutes(cfg *config.Config) *gin.Engine {
	router := gin.Default()

	router.Use(middleware.CORSMiddleware())

	// Handlers
	authHandler := handlers.NewAuthHandler(
		cfg.SpotifyClientID,
		cfg.SpotifyClientSecret,
		cfg.StateString,
		cfg.SpotifyRedirectURL,
		database.DB,
		cfg.JWTSecret,
	)
	healthHandler := handlers.NewHealthHandler()
	artistHandler := handlers.NewArtistHandler(database.DB)
	playlistHandler := handlers.NewPlaylistHandler(database.DB)
	concertHandler := handlers.NewConcertHandler(database.DB)

	authMiddleware := middleware.RequireAuth()

	api := router.Group("/api/v1")
	{
		api.GET("/health", healthHandler.Check)

		// =====================
		// AUTH
		// =====================
		auth := api.Group("/auth")
		{
			auth.POST("/spotify/token", authHandler.ExchangeCodeForToken)
			auth.GET("/me", middleware.RequireAuth(), authHandler.Me)
		}

		// =====================
		// TASTE
		// =====================
		taste := api.Group("/taste")
		taste.Use(authMiddleware)
		{
			// Artists
			taste.POST("/artists/sync", artistHandler.Sync)
			taste.GET("/artists", artistHandler.GetUserArtists)

			// Playlists
			taste.POST("/playlists", playlistHandler.Create)
			taste.GET("/playlists", playlistHandler.List)
			taste.GET("/playlists/:id", playlistHandler.Get)
			taste.POST("/playlists/:id/tracks", playlistHandler.AddTracks)
			taste.DELETE("/playlists/:id", playlistHandler.Delete)
		}

		// =====================
		// DISCOVERY
		// =====================
		concerts := api.Group("/concerts")
		concerts.Use(authMiddleware)
		{
			concerts.GET("", concertHandler.GetNearby)
			concerts.GET("/:id", concertHandler.GetByID)
		}
	}

	return router
}
