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

// batchSize keeps bulk inserts well under Postgres's 65535 bind-parameter limit.
const batchSize = 500

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
	ids, err := i.ImportArtists([]Artist{a})
	if err != nil {
		return uuid.Nil, err
	}
	return ids[0], nil
}

// ImportArtists returns canonical artist IDs in the same order as as, creating
// any missing rows in bulk.
func (i *Importer) ImportArtists(as []Artist) ([]uuid.UUID, error) {
	var missing []models.Artist
	queued := make(map[string]bool)
	for _, a := range as {
		key := Normalize(a.Name)
		if _, ok := i.artists[key]; ok || queued[key] {
			continue
		}
		queued[key] = true
		missing = append(missing, models.Artist{Name: a.Name, NormalizedName: key})
	}

	if len(missing) > 0 {
		// The no-op update makes Postgres return the existing row's ID on conflict.
		if err := i.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "normalized_name"}},
			DoUpdates: clause.AssignmentColumns([]string{"normalized_name"}),
		}).CreateInBatches(&missing, batchSize).Error; err != nil {
			return nil, err
		}
		for _, m := range missing {
			i.artists[m.NormalizedName] = m.ID
		}
	}

	ids := make([]uuid.UUID, len(as))
	var links []models.ExternalID
	for n, a := range as {
		ids[n] = i.artists[Normalize(a.Name)]
		links = i.appendLink(links, "artist", a.ExternalID, ids[n])
	}
	if err := i.saveLinks(links); err != nil {
		return nil, err
	}
	return ids, nil
}

// ImportTrack returns the canonical track ID. A track is identified by its
// normalized title plus its primary artist, so "Shame" by two different
// artists are two tracks, and the same song arriving from two providers is one.
func (i *Importer) ImportTrack(t Track) (uuid.UUID, error) {
	ids, err := i.ImportTracks([]Track{t})
	if err != nil {
		return uuid.Nil, err
	}
	return ids[0], nil
}

// ImportTracks returns canonical track IDs in the same order as ts, creating
// any missing tracks, artists and track_artists rows in bulk.
func (i *Importer) ImportTracks(ts []Track) ([]uuid.UUID, error) {
	keys := make([]string, len(ts))
	var missing []models.Track
	var missingSrc []Track
	queued := make(map[string]bool)
	for n, t := range ts {
		keys[n] = trackKey(t)
		if _, ok := i.tracks[keys[n]]; ok || queued[keys[n]] {
			continue
		}
		queued[keys[n]] = true
		missing = append(missing, models.Track{
			Name:         t.Title,
			Normalized:   Normalize(t.Title),
			CanonicalKey: keys[n],
		})
		missingSrc = append(missingSrc, t)
	}

	if len(missing) > 0 {
		if err := i.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "canonical_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"canonical_key"}),
		}).CreateInBatches(&missing, batchSize).Error; err != nil {
			return nil, err
		}

		var credits []Artist
		for _, t := range missingSrc {
			credits = append(credits, t.Artists...)
		}
		artistIDs, err := i.ImportArtists(credits)
		if err != nil {
			return nil, err
		}

		var joins []models.TrackArtist
		n := 0
		for k, t := range missingSrc {
			for range t.Artists {
				joins = append(joins, models.TrackArtist{TrackID: missing[k].ID, ArtistID: artistIDs[n]})
				n++
			}
		}
		if len(joins) > 0 {
			if err := i.db.Clauses(clause.OnConflict{DoNothing: true}).
				CreateInBatches(&joins, batchSize).Error; err != nil {
				return nil, err
			}
		}

		for _, m := range missing {
			i.tracks[m.CanonicalKey] = m.ID
		}
	}

	ids := make([]uuid.UUID, len(ts))
	var links []models.ExternalID
	for n, t := range ts {
		ids[n] = i.tracks[keys[n]]
		links = i.appendLink(links, "track", t.ExternalID, ids[n])
	}
	if err := i.saveLinks(links); err != nil {
		return nil, err
	}
	return ids, nil
}

func trackKey(t Track) string {
	primary := ""
	if len(t.Artists) > 0 {
		primary = t.Artists[0].Name
	}
	return Normalize(t.Title) + "|" + Normalize(primary)
}

// appendLink queues a record that this provider knows the entity under
// externalID, skipping ones already recorded or queued.
func (i *Importer) appendLink(links []models.ExternalID, entityType, externalID string, entityID uuid.UUID) []models.ExternalID {
	k := entityType + ":" + externalID
	if externalID == "" || i.linked[k] {
		return links
	}
	// Marked now so duplicates within a batch are skipped; saveLinks clears
	// these again if the insert fails.
	i.linked[k] = true
	return append(links, models.ExternalID{
		Provider:   i.provider,
		EntityType: entityType,
		ExternalID: externalID,
		EntityID:   entityID,
	})
}

func (i *Importer) saveLinks(links []models.ExternalID) error {
	if len(links) == 0 {
		return nil
	}
	if err := i.db.Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(&links, batchSize).Error; err != nil {
		for _, l := range links {
			delete(i.linked, l.EntityType+":"+l.ExternalID)
		}
		return err
	}
	return nil
}
