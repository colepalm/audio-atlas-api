package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"audio-atlas-api/models"
	"audio-atlas-api/services/ingest"
)

const (
	apiBase = "https://api.spotify.com/v1"

	// playlistWorkers bounds concurrent playlist fetches. Spotify rate limits
	// per app over a rolling window, so more isn't necessarily faster.
	playlistWorkers = 4
	maxRetries      = 4
	batchSize       = 500

	// requestInterval paces all Spotify requests from this process (~10/s).
	requestInterval = 100 * time.Millisecond
	// maxRetryAfter is the longest Retry-After we'll wait out. Spotify sends
	// much longer ones when an app's quota is exhausted; fail fast instead.
	maxRetryAfter = 2 * time.Minute
)

// Spotify's rate limit is per app, so every sync in the process shares one throttle.
var limiter = &throttle{interval: requestInterval}

// throttle spaces requests at least interval apart, and holds every caller
// while a Retry-After pause is in effect.
type throttle struct {
	interval time.Duration

	mu          sync.Mutex
	next        time.Time // earliest start for the next request
	pausedUntil time.Time
}

func (t *throttle) wait(ctx context.Context) error {
	for {
		t.mu.Lock()
		now := time.Now()
		var d time.Duration
		switch {
		case now.Before(t.pausedUntil):
			d = t.pausedUntil.Sub(now)
		case now.Before(t.next):
			d = t.next.Sub(now)
		default:
			t.next = now.Add(t.interval)
			t.mu.Unlock()
			return nil
		}
		t.mu.Unlock()

		if err := sleep(ctx, d); err != nil {
			return err
		}
	}
}

func (t *throttle) pause(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if until := time.Now().Add(d); until.After(t.pausedUntil) {
		t.pausedUntil = until
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SyncService fetches a user's Spotify data and hands it to the provider-neutral
// importer. It never writes artists or tracks directly.
type SyncService struct {
	DB     *gorm.DB
	Client *http.Client // authorized; see providerauth.Client
	UserID uuid.UUID

	imp *ingest.Importer
}

func NewSyncService(db *gorm.DB, userID uuid.UUID, client *http.Client) *SyncService {
	return &SyncService{
		DB:     db,
		Client: client,
		UserID: userID,
		imp:    ingest.NewImporter(db, "spotify"),
	}
}

func (s *SyncService) Run(ctx context.Context) {
	start := time.Now()
	log.Printf("[SpotifySync] Starting sync for user %s", s.UserID)

	if err := s.syncTopArtists(ctx); err != nil {
		log.Printf("[SpotifySync] syncTopArtists failed: %v", err)
	}
	if err := s.syncPlaylists(ctx); err != nil {
		log.Printf("[SpotifySync] syncPlaylists failed: %v", err)
	}
	if err := s.syncRecentlyPlayed(ctx); err != nil {
		log.Printf("[SpotifySync] syncRecentlyPlayed failed: %v", err)
	}

	log.Printf("[SpotifySync] Sync complete for user %s in %s", s.UserID, time.Since(start).Round(time.Millisecond))
}

// toIngestTrack maps Spotify's shape onto the neutral one.
func toIngestTrack(id, name string, artists []Artist) ingest.Track {
	t := ingest.Track{Title: name, ExternalID: id}
	for _, a := range artists {
		t.Artists = append(t.Artists, ingest.Artist{Name: a.Name, ExternalID: a.ID})
	}
	return t
}

// ------------------------------------------------------------------
// Top Artists
// ------------------------------------------------------------------
func (s *SyncService) syncTopArtists(ctx context.Context) error {
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
		if err := s.get(ctx, fmt.Sprintf("/me/top/artists?limit=50&time_range=%s", timeRange), &res); err != nil {
			return err
		}

		artists := make([]ingest.Artist, len(res.Items))
		for n, item := range res.Items {
			artists[n] = ingest.Artist{Name: item.Name, ExternalID: item.ID}
		}
		artistIDs, err := s.imp.ImportArtists(artists)
		if err != nil {
			return err
		}

		now := time.Now()
		var (
			genres      []models.ArtistGenre
			userArtists []models.UserArtist
			entries     []models.SnapshotArtist
			seen        = make(map[uuid.UUID]bool)
		)
		for rank, item := range res.Items {
			artistID := artistIDs[rank]
			// Two Spotify artists can normalize to the same name; one row per
			// artist keeps the bulk upserts from hitting the same key twice.
			if seen[artistID] {
				continue
			}
			seen[artistID] = true

			for _, genre := range item.Genres {
				genres = append(genres, models.ArtistGenre{ArtistID: artistID, Genre: genre, Source: "spotify"})
			}
			userArtists = append(userArtists, models.UserArtist{
				UserID:     s.UserID,
				ArtistID:   artistID,
				Source:     "spotify",
				PlayCount:  item.Popularity,
				LastSynced: now,
			})
			entries = append(entries, models.SnapshotArtist{
				ArtistID:  artistID,
				Rank:      rank + 1,
				PlayCount: item.Popularity,
			})
		}

		err = s.DB.Transaction(func(tx *gorm.DB) error {
			snapshot := models.ListeningSnapshot{
				UserID:    s.UserID,
				Source:    "spotify",
				TimeRange: timeRange,
				CreatedAt: now,
			}
			if err := tx.Create(&snapshot).Error; err != nil {
				return err
			}
			for n := range entries {
				entries[n].SnapshotID = snapshot.ID
			}

			if len(genres) > 0 {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
					CreateInBatches(&genres, batchSize).Error; err != nil {
					return err
				}
			}
			if len(userArtists) > 0 {
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "user_id"}, {Name: "artist_id"}},
					DoUpdates: clause.AssignmentColumns([]string{"play_count", "last_synced"}),
				}).CreateInBatches(&userArtists, batchSize).Error; err != nil {
					return err
				}
			}
			if len(entries) > 0 {
				if err := tx.CreateInBatches(&entries, batchSize).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// ------------------------------------------------------------------
// Playlists + Tracks
// ------------------------------------------------------------------
type spotifyPlaylist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SnapshotID string `json:"snapshot_id"`
}

type playlistItem struct {
	Track struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Artists []Artist `json:"artists"`
	} `json:"track"`
	AddedAt string `json:"added_at"`
}

type fetchedPlaylist struct {
	playlistID uuid.UUID
	src        spotifyPlaylist
	items      []playlistItem
	err        error
}

func (s *SyncService) syncPlaylists(ctx context.Context) error {
	type Response struct {
		Items []spotifyPlaylist `json:"items"`
		Next  *string           `json:"next"`
	}

	var all []spotifyPlaylist
	path := "/me/playlists?limit=50"
	for path != "" {
		var res Response
		if err := s.get(ctx, path, &res); err != nil {
			return err
		}
		all = append(all, res.Items...)
		path = s.nextPath(res.Next)
	}

	// Upsert playlist rows and keep only the ones whose contents changed since
	// the last sync. Spotify bumps snapshot_id on every edit.
	type job struct {
		playlistID uuid.UUID
		src        spotifyPlaylist
	}
	var jobs []job
	for _, item := range all {
		if item.ID == "" {
			continue
		}
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

		if item.SnapshotID != "" && playlist.SyncVersion != nil && *playlist.SyncVersion == item.SnapshotID {
			continue
		}
		jobs = append(jobs, job{playlistID: playlist.ID, src: item})
	}
	log.Printf("[SpotifySync] %d playlists, %d changed since last sync", len(all), len(jobs))

	// Fetch concurrently, import serially: the importer isn't goroutine-safe,
	// and the DB work is quick next to Spotify's round trips.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan job)
	results := make(chan fetchedPlaylist)
	for w := 0; w < playlistWorkers; w++ {
		go func() {
			for j := range jobCh {
				items, err := s.fetchPlaylistItems(ctx, j.src.ID)
				select {
				case results <- fetchedPlaylist{playlistID: j.playlistID, src: j.src, items: items, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobCh)
		for _, j := range jobs {
			select {
			case jobCh <- j:
			case <-ctx.Done():
				return
			}
		}
	}()

	for range jobs {
		var r fetchedPlaylist
		select {
		case r = <-results:
		case <-ctx.Done():
			return ctx.Err()
		}
		if r.err != nil {
			log.Printf("[SpotifySync] fetching playlist %s failed: %v", r.src.Name, r.err)
			continue
		}
		if err := s.importPlaylistItems(r); err != nil {
			log.Printf("[SpotifySync] importing playlist %s failed: %v", r.src.Name, err)
		}
	}

	return nil
}

func (s *SyncService) fetchPlaylistItems(ctx context.Context, spotifyPlaylistID string) ([]playlistItem, error) {
	type Response struct {
		Items []playlistItem `json:"items"`
		Next  *string        `json:"next"`
	}

	// fields trims the response to what we use; full track objects are large.
	q := url.Values{
		"limit":  {"100"},
		"fields": {"next,items(added_at,track(id,name,artists(id,name)))"},
	}
	path := fmt.Sprintf("/playlists/%s/tracks?%s", spotifyPlaylistID, q.Encode())

	var items []playlistItem
	for path != "" {
		var res Response
		if err := s.get(ctx, path, &res); err != nil {
			return nil, err
		}
		items = append(items, res.Items...)
		path = s.nextPath(res.Next)
	}
	return items, nil
}

func (s *SyncService) importPlaylistItems(r fetchedPlaylist) error {
	var (
		tracks   []ingest.Track
		addedAts []time.Time
	)
	for _, item := range r.items {
		if item.Track.ID == "" {
			continue // local or unavailable track
		}
		tracks = append(tracks, toIngestTrack(item.Track.ID, item.Track.Name, item.Track.Artists))

		addedAt := time.Now()
		if t, err := time.Parse(time.RFC3339, item.AddedAt); err == nil {
			addedAt = t
		}
		addedAts = append(addedAts, addedAt)
	}

	trackIDs, err := s.imp.ImportTracks(tracks)
	if err != nil {
		return err
	}

	return s.DB.Transaction(func(tx *gorm.DB) error {
		var existing []uuid.UUID
		if err := tx.Model(&models.PlaylistTrack{}).
			Where("playlist_id = ?", r.playlistID).
			Pluck("track_id", &existing).Error; err != nil {
			return err
		}
		had := make(map[uuid.UUID]bool, len(existing))
		for _, id := range existing {
			had[id] = true
		}

		// Only tracks new to this playlist get an import event, so re-syncing
		// doesn't keep inflating their weight.
		var (
			rows   []models.PlaylistTrack
			events []models.UserTrackEvent
			keep   = make(map[uuid.UUID]bool, len(trackIDs))
		)
		for n, id := range trackIDs {
			if keep[id] {
				continue
			}
			keep[id] = true
			if had[id] {
				continue
			}
			rows = append(rows, models.PlaylistTrack{PlaylistID: r.playlistID, TrackID: id, AddedAt: addedAts[n]})
			events = append(events, models.UserTrackEvent{
				UserID:  s.UserID,
				TrackID: id,
				Type:    "import",
				Source:  "spotify",
				Weight:  1.0,
			})
		}

		var removed []uuid.UUID
		for _, id := range existing {
			if !keep[id] {
				removed = append(removed, id)
			}
		}
		if len(removed) > 0 {
			if err := tx.Where("playlist_id = ? AND track_id IN ?", r.playlistID, removed).
				Delete(&models.PlaylistTrack{}).Error; err != nil {
				return err
			}
		}
		if len(rows) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
				CreateInBatches(&rows, batchSize).Error; err != nil {
				return err
			}
		}
		if len(events) > 0 {
			if err := tx.CreateInBatches(&events, batchSize).Error; err != nil {
				return err
			}
		}

		// Recorded last, so a failed import is retried next sync.
		if r.src.SnapshotID != "" {
			return tx.Model(&models.Playlist{}).Where("id = ?", r.playlistID).
				Update("sync_version", r.src.SnapshotID).Error
		}
		return nil
	})
}

// ------------------------------------------------------------------
// Recently Played
// ------------------------------------------------------------------
func (s *SyncService) syncRecentlyPlayed(ctx context.Context) error {
	type PlayedTrack struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Artists []Artist `json:"artists"`
	}

	type Item struct {
		Track    PlayedTrack `json:"track"`
		PlayedAt string      `json:"played_at"`
	}

	type Response struct {
		Items []Item `json:"items"`
	}

	var res Response
	if err := s.get(ctx, "/me/player/recently-played?limit=50", &res); err != nil {
		return err
	}

	var (
		tracks   []ingest.Track
		playedAt []time.Time
	)
	for _, item := range res.Items {
		if item.Track.ID == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, item.PlayedAt)
		if err != nil {
			t = time.Now()
		}
		tracks = append(tracks, toIngestTrack(item.Track.ID, item.Track.Name, item.Track.Artists))
		playedAt = append(playedAt, t)
	}
	if len(tracks) == 0 {
		return nil
	}

	trackIDs, err := s.imp.ImportTracks(tracks)
	if err != nil {
		return err
	}

	return s.DB.Transaction(func(tx *gorm.DB) error {
		// Spotify returns the same last-50 window every call; only plays newer
		// than the last one we recorded become events.
		var last time.Time
		if err := tx.Model(&models.UserTrackEvent{}).
			Where("user_id = ? AND source = ? AND type = ?", s.UserID, "spotify", "play").
			Select("COALESCE(MAX(created_at), 'epoch')").
			Scan(&last).Error; err != nil {
			return err
		}

		snapshot := models.ListeningSnapshot{
			UserID:    s.UserID,
			Source:    "spotify",
			TimeRange: "recent",
			CreatedAt: time.Now(),
		}
		if err := tx.Create(&snapshot).Error; err != nil {
			return err
		}

		var (
			entries []models.SnapshotTrack
			events  []models.UserTrackEvent
			seen    = make(map[uuid.UUID]bool)
		)
		for n, id := range trackIDs {
			if !seen[id] {
				seen[id] = true
				entries = append(entries, models.SnapshotTrack{SnapshotID: snapshot.ID, TrackID: id, Rank: len(entries) + 1})
			}
			if playedAt[n].After(last) {
				events = append(events, models.UserTrackEvent{
					UserID:    s.UserID,
					TrackID:   id,
					Type:      "play",
					Source:    "spotify",
					Weight:    1.0,
					CreatedAt: playedAt[n],
				})
			}
		}

		if err := tx.CreateInBatches(&entries, batchSize).Error; err != nil {
			return err
		}
		if len(events) > 0 {
			return tx.CreateInBatches(&events, batchSize).Error
		}
		return nil
	})
}

// ------------------------------------------------------------------
// HTTP helpers
// ------------------------------------------------------------------
func (s *SyncService) nextPath(next *string) string {
	if next == nil {
		return ""
	}
	return strings.TrimPrefix(*next, apiBase)
}

// get fetches path and decodes the JSON body into out, retrying on rate limits
// and transient server errors.
func (s *SyncService) get(ctx context.Context, path string, out interface{}) error {
	for attempt := 0; ; attempt++ {
		if err := limiter.wait(ctx); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiBase+path, nil)
		if err != nil {
			return err
		}

		resp, err := s.Client.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusOK {
			err := json.NewDecoder(resp.Body).Decode(out)
			resp.Body.Close()
			return err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retryable || attempt >= maxRetries {
			return fmt.Errorf("spotify %s returned %d: %s", path, resp.StatusCode, body)
		}

		wait := time.Duration(1<<attempt) * time.Second
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 {
			wait = time.Duration(secs) * time.Second
		}
		if wait > maxRetryAfter {
			return fmt.Errorf("spotify %s returned %d with Retry-After %s; giving up", path, resp.StatusCode, wait)
		}
		log.Printf("[SpotifySync] %s returned %d, pausing all requests for %s", path, resp.StatusCode, wait)

		// The next limiter.wait sleeps through this, along with every other worker.
		limiter.pause(wait)
	}
}
