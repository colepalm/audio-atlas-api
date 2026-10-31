package ingest

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"audio-atlas-api/models"
)

type Artist struct {
	Name       string
	ExternalID string // optional
}

type Track struct {
	Title      string
	Artists    []Artist // first entry is the primary artist
	ExternalID string   // optional
}

type Importer struct {
	db       *gorm.DB
	provider string

	// Per-import caches; an Importer is used by a single goroutine.
	artists map[string]uuid.UUID // normalized name -> artist ID
	tracks  map[string]uuid.UUID // canonical key -> track ID
	linked  map[string]bool      // entityType:externalID already recorded
}

func NewImporter(db *gorm.DB, provider string) *Importer {
	return &Importer{
		db:       db,
		provider: provider,
		artists:  make(map[string]uuid.UUID),
		tracks:   make(map[string]uuid.UUID),
		linked:   make(map[string]bool),
	}
}

func Normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ImportArtist returns the canonical artist ID, creating the row if needed.
func (i *Importer) ImportArtist(a Artist) (uuid.UUID, error) {
	key := Normalize(a.Name)

	id, ok := i.artists[key]
	if !ok {
		artist := models.Artist{Name: a.Name, NormalizedName: key}
		// The no-op update makes Postgres return the existing row's ID on conflict.
		if err := i.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "normalized_name"}},
			DoUpdates: clause.AssignmentColumns([]string{"normalized_name"}),
		}).Create(&artist).Error; err != nil {
			return uuid.Nil, err
		}
		id = artist.ID
		i.artists[key] = id
	}

	if err := i.link("artist", a.ExternalID, id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ImportTrack returns the canonical track ID. A track is identified by its
// normalized title plus its primary artist, so "Shame" by two different
// artists are two tracks, and the same song arriving from two providers is one.
func (i *Importer) ImportTrack(t Track) (uuid.UUID, error) {
	primary := ""
	if len(t.Artists) > 0 {
		primary = t.Artists[0].Name
	}
	key := Normalize(t.Title) + "|" + Normalize(primary)

	id, ok := i.tracks[key]
	if !ok {
		track := models.Track{
			Name:         t.Title,
			Normalized:   Normalize(t.Title),
			CanonicalKey: key,
		}
		if err := i.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "canonical_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"canonical_key"}),
		}).Create(&track).Error; err != nil {
			return uuid.Nil, err
		}
		id = track.ID

		for _, a := range t.Artists {
			artistID, err := i.ImportArtist(a)
			if err != nil {
				return uuid.Nil, err
			}
			if err := i.db.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.TrackArtist{TrackID: id, ArtistID: artistID}).Error; err != nil {
				return uuid.Nil, err
			}
		}
		i.tracks[key] = id
	}

	if err := i.link("track", t.ExternalID, id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// link records that this provider knows the entity under externalID.
func (i *Importer) link(entityType, externalID string, entityID uuid.UUID) error {
	if externalID == "" {
		return nil
	}
	k := entityType + ":" + externalID
	if i.linked[k] {
		return nil
	}
	if err := i.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ExternalID{
		Provider:   i.provider,
		EntityType: entityType,
		ExternalID: externalID,
		EntityID:   entityID,
	}).Error; err != nil {
		return err
	}
	i.linked[k] = true
	return nil
}
