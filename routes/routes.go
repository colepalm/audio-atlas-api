package routes

import (
	"audio-atlas-api/config"
	"audio-atlas-api/database"
	"audio-atlas-api/handlers"
	authHandler "audio-atlas-api/handlers/auth"
	"audio-atlas-api/handlers/providers"
	"audio-atlas-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(cfg *config.Config) *gin.Engine {
	router := gin.Default()

	router.Use(middleware.CORSMiddleware())

	// Handlers
	auth := authHandler.NewHandler(database.DB, cfg.JWTSecret)
	spotifyHandler := providers.NewSpotifyHandler(cfg.SpotifyOAuthConfig(), database.DB, cfg.FrontendURL)

	healthHandler := handlers.NewHealthHandler()
	artistHandler := handlers.NewArtistHandler(database.DB)
	playlistHandler := handlers.NewPlaylistHandler(database.DB)
	concertHandler := handlers.NewConcertHandler(database.DB)
	tasteHandler := handlers.NewTasteHandler(database.DB)

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
			authRoutes.POST("/refresh", auth.Refresh)

		}

		me := api.Group("/me")
		me.Use(authMiddleware)
		{
			me.GET("", auth.Me)
			me.GET("/recently-played", tasteHandler.RecentlyPlayed)  // reads from ListeningSnapshot/UserTrackEvent
			me.GET("/player/current", tasteHandler.NowPlaying)       // reads from your DB
			me.GET("/playlists", playlistHandler.List)               // already exists!
			me.GET("/recommendations", tasteHandler.Recommendations) // based on UserArtist/genres
			me.GET("/top/artists", tasteHandler.TopArtists)          // reads from UserArtist
			me.GET("/top/tracks", tasteHandler.TopTracks)            // reads from SnapshotTrack
			me.GET("/new-releases", tasteHandler.NewReleases)        // from Event/external source
			// TODO: me.PUT("", userHandler.Update)
			// TODO: me.GET("/stats", statsHandler.Get)
		}

		providersGroup := api.Group("/providers")
		{
			spotify := providersGroup.Group("/spotify")
			spotify.GET("/callback", spotifyHandler.Callback) // public - called by Spotify

			spotifyAuth := spotify.Group("")
			spotifyAuth.Use(authMiddleware)
			{
				spotifyAuth.GET("/connect", spotifyHandler.Connect)
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
