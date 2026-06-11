package database

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"audio-atlas-api/models"
)

var DB *gorm.DB

func InitDatabase() error {
	// Get connection string from environment
	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	// Try with explicit config
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	DB = db

	// Test the connection
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Println("Successfully connected to database!")

	if err := db.AutoMigrate(
		&models.User{},
		&models.ProviderAccount{},
		&models.Artist{},
		&models.ArtistGenre{},
		&models.UserArtist{},
		&models.Track{},
		&models.TrackArtist{},
		&models.Playlist{},
		&models.PlaylistTrack{},
		&models.ListeningSnapshot{},
		&models.SnapshotArtist{},
		&models.SnapshotTrack{},
		&models.Event{},
		&models.EventArtist{},
		&models.UserEventRecommendation{},
		&models.UserTrackEvent{},
		&models.RefreshToken{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	fmt.Println("Database migrated successfully!")
	return nil
}
