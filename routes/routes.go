package routes

import (
	"audio-atlas-api/handlers/providers"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"audio-atlas-api/config"
	"audio-atlas-api/database"
	"audio-atlas-api/handlers"
	authHandler "audio-atlas-api/handlers/auth"
	"audio-atlas-api/middleware"
)

func SetupRoutes(cfg *config.Config) *gin.Engine {
	router := gin.Default()

	router.Use(middleware.CORSMiddleware())

	// Handlers
	auth := authHandler.NewHandler(database.DB, cfg.JWTSecret)
	spotifyHandler := providers.NewSpotifyHandler(
		&oauth2.Config{
			RedirectURL:  cfg.SpotifyRedirectURL,
			ClientID:     cfg.SpotifyClientID,
			ClientSecret: cfg.SpotifyClientSecret,
			Scopes: []string{
				"user-top-read",
				"user-read-email",
			},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.spotify.com/authorize",
				TokenURL: "https://accounts.spotify.com/api/token",
			},
		},
		database.DB,
	)

	healthHandler := handlers.NewHealthHandler()
	artistHandler := handlers.NewArtistHandler(database.DB)
	playlistHandler := handlers.NewPlaylistHandler(database.DB)
	concertHandler := handlers.NewConcertHandler(database.DB)

	authMiddleware := middleware.RequireAuth([]byte(cfg.JWTSecret))

	api := router.Group("/api/v1")
	{
		api.GET("/health", healthHandler.Check)

		// =====================
		// AUTH
		// =====================
		authRoutes := api.Group("/auth")
		{
			authRoutes.POST("/register", auth.Register)
			authRoutes.POST("/login", auth.Login)

			authRoutes.GET("/me", authMiddleware, auth.Me)
		}

		providersGroup := api.Group("/providers")
		providersGroup.Use(authMiddleware)
		{
			spotify := providersGroup.Group("/spotify")
			{
				spotify.GET("/connect", spotifyHandler.Connect)
				spotify.GET("/callback", spotifyHandler.Callback)
			}
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
