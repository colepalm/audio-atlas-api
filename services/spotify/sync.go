package spotify

import (
	"log"

	"github.com/google/uuid"
)

func SyncUser(userID uuid.UUID, accessToken string) {
	log.Println("Syncing Spotify data for user:", userID)

	// TODO:
	// 1. Call Spotify API (/me/top/artists, /me/top/tracks)
	// 2. Upsert artists
	// 3. Upsert tracks
	// 4. Emit UserTrackEvents
}
