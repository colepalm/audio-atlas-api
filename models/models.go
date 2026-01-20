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

type Concert struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	ArtistID   uuid.UUID `gorm:"type:uuid;not null"`
	Venue      string    `gorm:"not null"`
	City       string    `gorm:"not null"`
	Date       time.Time `gorm:"not null"`
	URL        *string
	ExternalID *string
	FetchedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP"`

	// Relationships
	Artist Artist `gorm:"foreignKey:ArtistID;constraint:OnDelete:CASCADE"`
}
