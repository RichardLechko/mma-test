package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
	"mma-scheduler/config"
	"mma-scheduler/internal/services"
	"mma-scheduler/pkg/scrapers"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Load configuration
	if err := config.LoadConfig("config/config.json"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Configure database connection
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

	// Open database connection
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Ping database to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Create scraper
	scraper := scrapers.NewUFCEventScraper()

	// Scrape events with enhanced data
	events, err := scraper.ScrapeEvents(ctx)
	if err != nil {
		log.Fatalf("Failed to scrape events: %v", err)
	}

	log.Printf("Found %d events from UFC.com", len(events))

	// Create event service for database operations
	eventService := services.NewEventService(db)

	// Save events to database
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer dbCancel()

	insertCount := 0
	for _, event := range events {
		if err := eventService.CreateEvent(dbCtx, event); err != nil {
			log.Printf("Error saving event %s: %v", event.Name, err)
			continue
		}
		
		insertCount++
	}

	log.Printf("Scraping completed! Saved %d events.", insertCount)
}