package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
	"mma-scheduler/config"
	"mma-scheduler/internal/models"
	"mma-scheduler/internal/services"
	"mma-scheduler/pkg/scrapers"
)

func main() {
	// Set up logging
	log.Println("🚀 Starting Event Scraper")
	startTime := time.Now()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Load configuration
	if err := config.LoadConfig("config/config.json"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database")

	// Create scraper with configuration
	scraperConfig := &scrapers.ScraperConfig{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	}
	scraper := scrapers.NewUFCEventScraper(scraperConfig)

	// Create context with timeout for scraping
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Scrape events
	log.Println("Scraping events from UFC.com...")
	events, err := scraper.ScrapeEvents(ctx)
	if err != nil {
		log.Fatalf("Failed to scrape events: %v", err)
	}

	log.Printf("Found %d events from UFC.com", len(events))

	// Create database context with timeout for storage operations
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer dbCancel()

	// Create event service for database operations
	eventService := services.NewEventService(db)

	// Use a worker pool to save events to the database concurrently
	log.Println("Saving events to database...")
	saveEvents(dbCtx, events, eventService)

	log.Printf("🏁 Scraping completed in %v!", time.Since(startTime).Round(time.Second))
}

// connectToDatabase establishes a connection to the database
func connectToDatabase() (*sql.DB, error) {
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
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	// Ping database to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// saveEvents saves events to the database using a worker pool
func saveEvents(ctx context.Context, events []*models.Event, eventService *services.EventService) {
	// Set up concurrency control
	const maxWorkers = 10 // Max concurrent database operations
	sem := make(chan struct{}, maxWorkers)
	
	// For tracking results
	var wg sync.WaitGroup
	var statsMutex sync.Mutex
	insertCount := 0
	errorCount := 0
	
	// Process events concurrently
	for _, event := range events {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore
		
		go func(evt *models.Event) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore
			
			// Create event in database
			if err := eventService.CreateEvent(ctx, evt); err != nil {
				log.Printf("Error saving event %s: %v", evt.Name, err)
				
				statsMutex.Lock()
				errorCount++
				statsMutex.Unlock()
				return
			}
			
			statsMutex.Lock()
			insertCount++
			statsMutex.Unlock()
		}(event)
	}
	
	// Wait for all events to be processed
	wg.Wait()
	
	// Log results
	log.Printf("Database operation completed! Saved %d events, %d errors.", insertCount, errorCount)
}