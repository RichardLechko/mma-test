package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	if err := config.LoadConfig("config/config.json"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	dbConfig := config.GetDatabaseConfig()

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=require&pool_max_conns=%d&pool_min_conns=%d&statement_timeout=60000",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Database,
		dbConfig.MaxOpenConns,
		dbConfig.MaxIdleConns,
	)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Ensure the events table has the necessary columns
	updateSchemaCtx, updateSchemaCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer updateSchemaCancel()

	if err := updateEventSchema(updateSchemaCtx, db); err != nil {
		log.Printf("Warning: Failed to update event schema: %v", err)
	}

	// Create UFC event scraper with proper configuration
	scraperConfig := scrapers.DefaultConfig()
	// Set appropriate user agent and timeouts
	scraperConfig.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36"
	scraperConfig.Timeout = 30 * time.Second
	
	scraper := scrapers.NewUFCEventScraper(scraperConfig)

	// Set a longer timeout for scraping
	scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer scrapeCancel()

	log.Println("Scraping events directly from UFC.com...")
	
	// Scrape up to 50 pages (adjust if needed)
	maxPages := 50
	events, err := scraper.ScrapeEvents(scrapeCtx, maxPages)
	if err != nil {
		log.Fatalf("Error scraping events: %v", err)
	}

	log.Printf("Found %d events on UFC.com", len(events))

	// Save events to database
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer dbCancel()

	insertCount, err := scraper.SaveEvents(dbCtx, db, events)
	if err != nil {
		log.Printf("Error saving events: %v", err)
	}

	log.Printf("Scraping completed! Saved %d events.", insertCount)
}

// updateEventSchema ensures the events table has all necessary columns
func updateEventSchema(ctx context.Context, db *sql.DB) error {
	// Check if events table exists
	var tableExists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'events'
		)
	`).Scan(&tableExists)

	if err != nil {
		return fmt.Errorf("error checking if events table exists: %v", err)
	}

	// Create events table if it doesn't exist
	if !tableExists {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE events (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				name VARCHAR(255) UNIQUE NOT NULL,
				event_date TIMESTAMP,
				venue VARCHAR(255),
				location VARCHAR(255), 
				city VARCHAR(255),
				country VARCHAR(255),
				event_type VARCHAR(100),
				status VARCHAR(50),
				ufc_url VARCHAR(255),
				wiki_url VARCHAR(255),
				created_at TIMESTAMP NOT NULL,
				updated_at TIMESTAMP NOT NULL
			)
		`)
		if err != nil {
			return fmt.Errorf("error creating events table: %v", err)
		}
		log.Println("Created events table")
		return nil
	}

	// Add columns if they don't exist
	columns := []struct {
		name     string
		dataType string
	}{
		{"event_type", "VARCHAR(100)"},
		{"ufc_url", "VARCHAR(255)"},
		{"location", "VARCHAR(255)"},
		{"status", "VARCHAR(50)"},
	}

	for _, col := range columns {
		var columnExists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_schema = 'public' 
				AND table_name = 'events' 
				AND column_name = $1
			)
		`, col.name).Scan(&columnExists)

		if err != nil {
			return fmt.Errorf("error checking if column %s exists: %v", col.name, err)
		}

		if !columnExists {
			_, err := db.ExecContext(ctx, fmt.Sprintf(`
				ALTER TABLE events 
				ADD COLUMN %s %s
			`, col.name, col.dataType))
			
			if err != nil {
				return fmt.Errorf("error adding column %s: %v", col.name, err)
			}
			log.Printf("Added column %s to events table", col.name)
		}
	}

	return nil
}