package database

import (
	"os"

	"github.com/supabase-community/supabase-go"
)

var Client *supabase.Client

func InitSupabase() error {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY") // Use service_role key for backend

	client, err := supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		return err
	}

	Client = client
	return nil
}
