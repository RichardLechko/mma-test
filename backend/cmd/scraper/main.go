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

	config := scrapers.DefaultConfig()
	scraper := scrapers.NewEventScraper(config)

	log.Println("Scraping events from Wikipedia...")
	events, err := scraper.ScrapeEvents()
	if err != nil {
		log.Fatalf("Error scraping events: %v", err)
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer dbCancel()

	// Ensure the events table has a status column
	_, err = db.ExecContext(dbCtx, `
	ALTER TABLE events 
	ADD COLUMN IF NOT EXISTS status VARCHAR(50)
	`)
	if err != nil {
		log.Printf("Warning: Failed to add status column: %v", err)
	}

	insertCount := 0
    for _, event := range events {
        query := `
        INSERT INTO events (
            name, event_date, venue, city, country, status, wiki_url,
            created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9
        )
        ON CONFLICT (name) 
        DO UPDATE SET
            event_date = EXCLUDED.event_date,
            venue = EXCLUDED.venue,
            city = EXCLUDED.city,
            country = EXCLUDED.country,
            status = EXCLUDED.status,
            wiki_url = EXCLUDED.wiki_url,
            updated_at = EXCLUDED.updated_at
        RETURNING id`

		now := time.Now()
        var eventID string

        err = db.QueryRowContext(dbCtx, query,
            event.Name,
            event.Date,
            event.Venue,
            event.City,
            event.Country,
            event.Status,
            event.WikiURL, // Make sure this is included
            now,
            now,
        ).Scan(&eventID)

        if err != nil {
            log.Printf("Failed to save event %s: %v", event.Name, err)
            continue
        }
        
        insertCount++
    }

	log.Printf("Scraping completed! Saved %d events.", insertCount)
}
