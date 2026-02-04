package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Email     string    `gorm:"uniqueIndex;not null"`
	Location  string
	CreatedAt time.Time
}

type Artist struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name           string    `gorm:"not null"`
	NormalizedName string    `gorm:"uniqueIndex;not null"`
	SpotifyID      *string
	MusicbrainzID  *string
	LastfmName     *string
	CreatedAt      time.Time
}

type UserArtist struct {
	UserID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	ArtistID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Source     string    `gorm:"not null"` // 'spotify', 'lastfm', 'upload'
	PlayCount  int       `gorm:"default:0"`
	LastSynced time.Time `gorm:"default:CURRENT_TIMESTAMP"`

	// Relationships
	User   User   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Artist Artist `gorm:"foreignKey:ArtistID;constraint:OnDelete:CASCADE"`
}

type Event struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Name       string
	Venue      string
	City       string
	Latitude   *float64
	Longitude  *float64
	Date       time.Time
	Source     string // songkick, bandsintown
	ExternalID string
	URL        *string
	FetchedAt  time.Time
}

type EventArtist struct {
	EventID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ArtistID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

type UserEventRecommendation struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	EventID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Reason    string    // "Top artist", "Similar artist", etc
	Score     float64
	CreatedAt time.Time
}

type ListeningSnapshot struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null"`
	Source    string    `gorm:"not null"` // spotify, lastfm
	TimeRange string    `gorm:"not null"` // short/medium/long
	CreatedAt time.Time

	// Relationships
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

type SnapshotArtist struct {
	SnapshotID uuid.UUID `gorm:"type:uuid;primaryKey"`
	ArtistID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Rank       int
	PlayCount  int
}

type Playlist struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Source    string // manual, spotify, import
	CreatedAt time.Time
}

type PlaylistTrack struct {
	PlaylistID uuid.UUID
	TrackID    uuid.UUID
	Position   int
}

type Track struct {
	ID         uuid.UUID
	Title      string
	ArtistName string
	ArtistID   *uuid.UUID
	Source     string
	ExternalID string
}
