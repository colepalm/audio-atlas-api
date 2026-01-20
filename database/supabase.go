package database

import (
	"github.com/supabase-community/supabase-go"
	"os"
)

var Client *supabase.Client

func InitSupabase() error {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")

	client, err := supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		return err
	}

	Client = client
	return nil
}
