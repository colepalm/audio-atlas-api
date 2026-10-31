package spotify

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"audio-atlas-api/models"
	"audio-atlas-api/services/ingest"
)

type SpotifyArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SyncService fetches a user's Spotify data and hands it to the provider-neutral
// importer. It never writes artists or tracks directly.
type SyncService struct {
	DB          *gorm.DB
	AccessToken string
	UserID      uuid.UUID

	imp *ingest.Importer
}

func NewSyncService(db *gorm.DB, userID uuid.UUID, accessToken string) *SyncService {
	return &SyncService{
		DB:          db,
		AccessToken: accessToken,
		UserID:      userID,
		imp:         ingest.NewImporter(db, "spotify"),
	}
}

func (s *SyncService) Run() {
	log.Printf("[SpotifySync] Starting sync for user %s", s.UserID)

	if err := s.syncTopArtists(); err != nil {
		log.Printf("[SpotifySync] syncTopArtists failed: %v", err)
	}
	if err := s.syncPlaylists(); err != nil {
		log.Printf("[SpotifySync] syncPlaylists failed: %v", err)
	}
	if err := s.syncRecentlyPlayed(); err != nil {
		log.Printf("[SpotifySync] syncRecentlyPlayed failed: %v", err)
	}

	log.Printf("[SpotifySync] Sync complete for user %s", s.UserID)
}

// toIngestTrack maps Spotify's shape onto the neutral one.
func toIngestTrack(id, name string, artists []SpotifyArtist) ingest.Track {
	t := ingest.Track{Title: name, ExternalID: id}
	for _, a := range artists {
		t.Artists = append(t.Artists, ingest.Artist{Name: a.Name, ExternalID: a.ID})
	}
	return t
}

// ------------------------------------------------------------------
// Top Artists
// ------------------------------------------------------------------
func (s *SyncService) syncTopArtists() error {
	type TopArtist struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Genres     []string `json:"genres"`
		Popularity int      `json:"popularity"`
	}

	type Response struct {
		Items []TopArtist `json:"items"`
	}

	for _, timeRange := range []string{"short_term", "medium_term", "long_term"} {
		var res Response
		if err := s.get(fmt.Sprintf("/me/top/artists?limit=50&time_range=%s", timeRange), &res); err != nil {
			return err
		}

		snapshot := models.ListeningSnapshot{
			UserID:    s.UserID,
			Source:    "spotify",
			TimeRange: timeRange,
			CreatedAt: time.Now(),
		}
		if err := s.DB.Create(&snapshot).Error; err != nil {
			return err
		}

		for rank, item := range res.Items {
			artistID, err := s.imp.ImportArtist(ingest.Artist{Name: item.Name, ExternalID: item.ID})
			if err != nil {
				return err
			}

			// Upsert genres
			for _, genre := range item.Genres {
				s.DB.Clauses(clause.OnConflict{DoNothing: true}).
					Create(&models.ArtistGenre{
						ArtistID: artistID,
						Genre:    genre,
						Source:   "spotify",
					})
			}

			s.DB.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}, {Name: "artist_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"play_count", "last_synced"}),
			}).Create(&models.UserArtist{
				UserID:     s.UserID,
				ArtistID:   artistID,
				Source:     "spotify",
				PlayCount:  item.Popularity,
				LastSynced: time.Now(),
			})

			// Snapshot entry
			s.DB.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.SnapshotArtist{
					SnapshotID: snapshot.ID,
					ArtistID:   artistID,
					Rank:       rank + 1,
					PlayCount:  item.Popularity,
				})
		}
	}

	return nil
}

// ------------------------------------------------------------------
// Playlists + Tracks
// ------------------------------------------------------------------
func (s *SyncService) syncPlaylists() error {
	type SpotifyPlaylist struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	type Response struct {
		Items []SpotifyPlaylist `json:"items"`
		Next  *string           `json:"next"`
	}

	path := "/me/playlists?limit=50"
	for path != "" {
		var res Response
		if err := s.get(path, &res); err != nil {
			return err
		}

		for _, item := range res.Items {
			externalID := item.ID
			playlist := models.Playlist{
				UserID:     s.UserID,
				Name:       item.Name,
				Source:     "spotify",
				ExternalID: &externalID,
			}

			if err := s.DB.
				Where("external_id = ? AND user_id = ?", externalID, s.UserID).
				Assign(models.Playlist{Name: item.Name}).
				FirstOrCreate(&playlist).Error; err != nil {
				return err
			}

			if err := s.syncPlaylistTracks(playlist.ID, item.ID); err != nil {
				log.Printf("[SpotifySync] syncPlaylistTracks %s failed: %v", item.Name, err)
			}
		}

		path = s.nextPath(res.Next)
	}

	return nil
}

func (s *SyncService) syncPlaylistTracks(playlistID uuid.UUID, spotifyPlaylistID string) error {
	type PlaylistTrack struct {
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Artists []SpotifyArtist `json:"artists"`
	}

	type Item struct {
		Track   PlaylistTrack `json:"track"`
		AddedAt string        `json:"added_at"`
	}

	type Response struct {
		Items []Item  `json:"items"`
		Next  *string `json:"next"`
	}

	path := fmt.Sprintf("/playlists/%s/tracks?limit=100", spotifyPlaylistID)
	for path != "" {
		var res Response
		if err := s.get(path, &res); err != nil {
			return err
		}

		for _, item := range res.Items {
			if item.Track.ID == "" {
				continue // local or unavailable track
			}

			trackID, err := s.imp.ImportTrack(toIngestTrack(item.Track.ID, item.Track.Name, item.Track.Artists))
			if err != nil {
				return err
			}

			addedAt := time.Now()
			if t, err := time.Parse(time.RFC3339, item.AddedAt); err == nil {
				addedAt = t
			}

			s.DB.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&models.PlaylistTrack{
					PlaylistID: playlistID,
					TrackID:    trackID,
					AddedAt:    addedAt,
				})

			s.DB.Create(&models.UserTrackEvent{
				UserID:  s.UserID,
				TrackID: trackID,
				Type:    "import",
				Source:  "spotify",
				Weight:  1.0,
			})
		}

		path = s.nextPath(res.Next)
	}

	return nil
}

// ------------------------------------------------------------------
// Recently Played
// ------------------------------------------------------------------
func (s *SyncService) syncRecentlyPlayed() error {
	type PlayedTrack struct {
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Artists []SpotifyArtist `json:"artists"`
	}

	type Item struct {
		Track    PlayedTrack `json:"track"`
		PlayedAt string      `json:"played_at"`
	}

	type Response struct {
		Items []Item `json:"items"`
	}

	var res Response
	if err := s.get("/me/player/recently-played?limit=50", &res); err != nil {
		return err
	}

	if len(res.Items) == 0 {
		return nil
	}

	snapshot := models.ListeningSnapshot{
		UserID:    s.UserID,
		Source:    "spotify",
		TimeRange: "recent",
		CreatedAt: time.Now(),
	}
	if err := s.DB.Create(&snapshot).Error; err != nil {
		return err
	}

	for rank, item := range res.Items {
		if item.Track.ID == "" {
			continue
		}

		trackID, err := s.imp.ImportTrack(toIngestTrack(item.Track.ID, item.Track.Name, item.Track.Artists))
		if err != nil {
			return err
		}

		s.DB.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&models.SnapshotTrack{
				SnapshotID: snapshot.ID,
				TrackID:    trackID,
				Rank:       rank + 1,
			})

		s.DB.Create(&models.UserTrackEvent{
			UserID:  s.UserID,
			TrackID: trackID,
			Type:    "play",
			Source:  "spotify",
			Weight:  1.0,
		})
	}

	return nil
}

// ------------------------------------------------------------------
// HTTP helpers
// ------------------------------------------------------------------
func (s *SyncService) nextPath(next *string) string {
	if next == nil {
		return ""
	}
	return strings.TrimPrefix(*next, "https://api.spotify.com/v1")
}

func (s *SyncService) get(path string, out interface{}) error {
	req, err := http.NewRequest("GET", "https://api.spotify.com/v1"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("spotify %s returned %d: %s", path, resp.StatusCode, body)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
