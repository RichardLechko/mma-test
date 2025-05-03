package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/robfig/cron/v3"
	
	"mma-scheduler/config"
	"mma-scheduler/internal/models"
	"mma-scheduler/internal/services"
	"mma-scheduler/pkg/scrapers"
)

type ScraperApp struct {
	db             *sql.DB
	eventService   *services.EventService
	scraperConfig  *scrapers.ScraperConfig
	logger         *log.Logger
}

func main() {
	fullScrapeFlag := flag.Bool("full", false, "Run a full scrape and reset the database")
	cronFlag := flag.Bool("cron", false, "Run as a cron job service")
	flag.Parse()

	logger := log.New(os.Stdout, "SCRAPER: ", log.LstdFlags)
	logger.Println("🚀 Starting Event Scraper")

	if err := godotenv.Load(); err != nil {
		logger.Printf("Warning: .env file not found: %v", err)
	}

	if err := config.LoadConfig("config/config.json"); err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := connectToDatabase()
	if err != nil {
		logger.Fatalf("Database connection error: %v", err)
	}
	
	app := &ScraperApp{
		db:           db,
		eventService: services.NewEventService(db),
		scraperConfig: &scrapers.ScraperConfig{
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		},
		logger: logger,
	}

	if *fullScrapeFlag {
		logger.Println("Running full scrape mode")
		app.runFullScrape()
		return
	} else if *cronFlag {
		logger.Println("Starting cron service mode")
		app.runCronService()
		return
	} else {
		logger.Println("Running recent update (last 2 months)")
		app.runRecentUpdate()
		return
	}
}

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

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func (app *ScraperApp) runFullScrape() {
	startTime := time.Now()
	app.logger.Println("Starting full database reset and event scrape")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	app.logger.Println("Resetting database tables...")
	if err := app.resetDatabase(ctx); err != nil {
		app.logger.Fatalf("Failed to reset database: %v", err)
	}

	scraper := scrapers.NewUFCEventScraper(app.scraperConfig)
	
	app.logger.Println("Scraping all events from UFC.com...")
	events, err := scraper.ScrapeEvents(ctx)
	if err != nil {
		app.logger.Fatalf("Failed to scrape events: %v", err)
	}

	app.logger.Printf("Found %d total events", len(events))

	app.saveEvents(ctx, events)

	app.logger.Printf("🏁 Full scrape completed in %v!", time.Since(startTime).Round(time.Second))
}

func (app *ScraperApp) runRecentUpdate() {
    startTime := time.Now()
    app.logger.Println("Starting update for recent events")

    // Calculate cutoff date - last 2 months
    twoMonthsAgo := time.Now().AddDate(0, -2, 0)
    app.logger.Printf("Using cutoff date: %s", twoMonthsAgo.Format("2006-01-02"))

    // Create context with timeout for scraping
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()

    // Create scraper
    scraper := scrapers.NewUFCEventScraper(app.scraperConfig)
    
    // Use the ScrapeRecentEvents function that stops when it finds old events
    app.logger.Println("Scraping recent events from UFC.com...")
    recentEvents, err := scraper.ScrapeRecentEvents(ctx, twoMonthsAgo)
    if err != nil {
        app.logger.Printf("Failed to scrape recent events: %v", err)
        return
    }

    app.logger.Printf("Found %d events within the last 2 months", len(recentEvents))

    // Save the recent events to database
    app.saveEvents(ctx, recentEvents)

    app.logger.Printf("🏁 Recent update completed in %v!", time.Since(startTime).Round(time.Second))
}

func (app *ScraperApp) runCronService() {
	app.logger.Println("Starting cron service for event updates")

	c := cron.New(
		cron.WithLogger(cron.VerbosePrintfLogger(app.logger)),
		cron.WithLocation(time.UTC),
	)

	if _, err := c.AddFunc("0 2 * * 1", app.runRecentUpdate); err != nil {
		app.logger.Fatalf("Failed to add weekly update job: %v", err)
	}
	
	if _, err := c.AddFunc("0 3 1 * *", app.runFullScrape); err != nil {
		app.logger.Fatalf("Failed to add monthly full update job: %v", err)
	}

	c.Start()
	app.logger.Println("Cron scheduler started with the following schedule:")
	app.logger.Println("- Weekly event updates every Monday at 02:00 UTC")
	app.logger.Println("- Monthly full scrape on the 1st at 03:00 UTC")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	app.logger.Println("Shutting down scraper service...")
	
	c.Stop()
	
	if err := app.db.Close(); err != nil {
		app.logger.Printf("Error closing database connection: %v", err)
	}
	
	app.logger.Println("Scraper service stopped gracefully")
}

func (app *ScraperApp) resetDatabase(ctx context.Context) error {
	queries := []string{
		"TRUNCATE TABLE fights CASCADE",
		"TRUNCATE TABLE events CASCADE",
		"TRUNCATE TABLE fighters CASCADE",
		"TRUNCATE TABLE fighter_rankings CASCADE",
	}
	
	for _, query := range queries {
		if _, err := app.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}
	
	return nil
}

func (app *ScraperApp) saveEvents(ctx context.Context, events []*models.Event) {
	const maxWorkers = 10
	sem := make(chan struct{}, maxWorkers)
	
	var wg sync.WaitGroup
	var statsMutex sync.Mutex
	insertCount := 0
	errorCount := 0
	
	for _, event := range events {
		wg.Add(1)
		sem <- struct{}{}
		
		go func(evt *models.Event) {
			defer wg.Done()
			defer func() { <-sem }()
			
			if err := app.eventService.CreateEvent(ctx, evt); err != nil {
				app.logger.Printf("Error saving event %s: %v", evt.Name, err)
				
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
	
	wg.Wait()
	
	app.logger.Printf("Database operation completed! Saved %d events, %d errors.", insertCount, errorCount)
}