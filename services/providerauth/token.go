package providerauth

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"audio-atlas-api/models"
)

const requestTimeout = 30 * time.Second

// Client returns an HTTP client authorized as account. Expired access tokens are
// refreshed automatically and written back to the provider_accounts row, so the
// next sync starts from a valid token.
func Client(ctx context.Context, db *gorm.DB, cfg *oauth2.Config, account *models.ProviderAccount) *http.Client {
	// Used for the refresh request itself.
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: requestTimeout})

	tok := &oauth2.Token{
		AccessToken:  account.AccessToken,
		RefreshToken: account.RefreshToken,
		Expiry:       account.Expiry,
		TokenType:    "Bearer",
	}

	client := oauth2.NewClient(ctx, &persistingSource{
		db:        db,
		accountID: account.ID,
		src:       cfg.TokenSource(ctx, tok),
		last:      tok.AccessToken,
	})
	client.Timeout = requestTimeout
	return client
}

// persistingSource saves a token to the database whenever the wrapped source
// hands back a new one.
type persistingSource struct {
	db        *gorm.DB
	accountID uuid.UUID
	src       oauth2.TokenSource

	mu   sync.Mutex
	last string
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	tok, err := p.src.Token()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if tok.AccessToken == p.last {
		return tok, nil
	}

	updates := map[string]interface{}{
		"access_token": tok.AccessToken,
		"expiry":       tok.Expiry,
	}
	// Providers may rotate the refresh token; keep the old one if they don't.
	if tok.RefreshToken != "" {
		updates["refresh_token"] = tok.RefreshToken
	}
	if err := p.db.Model(&models.ProviderAccount{}).Where("id = ?", p.accountID).Updates(updates).Error; err != nil {
		// The token still works for this run; the next run will just refresh again.
		log.Printf("[providerauth] failed to save refreshed token for account %s: %v", p.accountID, err)
	}
	p.last = tok.AccessToken
	return tok, nil
}
