package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Location     string
	CreatedAt    time.Time
}

type UserProvider struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index"`
	Provider       string    `gorm:"not null"` // spotify, lastfm
	ProviderUserID string    `gorm:"not null;index"`
	AccessToken    string
	RefreshToken   string
	TokenExpiry    time.Time
	CreatedAt      time.Time
}

type ProviderAccount struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID uuid.UUID `gorm:"index;not null"`

	Provider       string `gorm:"index:idx_provider_user,unique"`
	ProviderUserID string `gorm:"index:idx_provider_user,unique"`

	AccessToken  string
	RefreshToken string
	Expiry       time.Time

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
	EventID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	ArtistID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	Headliner bool
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
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID     uuid.UUID `gorm:"type:uuid;index;not null"`
	Name       string    `gorm:"not null"`
	Source     string    `gorm:"not null"` // spotify, manual, etc
	ExternalID *string
	CreatedAt  time.Time

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

type PlaylistTrack struct {
	PlaylistID uuid.UUID `gorm:"primaryKey"`
	TrackID    uuid.UUID `gorm:"primaryKey"`

	Playlist Playlist `gorm:"foreignKey:PlaylistID;constraint:OnDelete:CASCADE"`
	Track    Track    `gorm:"foreignKey:TrackID;constraint:OnDelete:CASCADE"`
	AddedAt  time.Time
}

type Track struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	Name          string    `gorm:"not null"`
	Normalized    string    `gorm:"index"`
	ArtistID      uuid.UUID
	MusicbrainzID *string
	CreatedAt     time.Time
}

type TrackArtist struct {
	TrackID  uuid.UUID `gorm:"primaryKey"`
	ArtistID uuid.UUID `gorm:"primaryKey"`

	Track  Track  `gorm:"foreignKey:TrackID;constraint:OnDelete:CASCADE"`
	Artist Artist `gorm:"foreignKey:ArtistID;constraint:OnDelete:CASCADE"`
}

type UserTrackEvent struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID  uuid.UUID `gorm:"type:uuid;index;not null"`
	TrackID uuid.UUID `gorm:"type:uuid;index;not null"`

	Type   string  `gorm:"not null"` // playlist_add, liked, play, import
	Source string  `gorm:"not null"` // manual, spotify, csv
	Weight float64 `gorm:"default:1"`

	CreatedAt time.Time
}
