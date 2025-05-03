package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/robfig/cron/v3"
	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"
)

type ScraperApp struct {
	db     *sql.DB
	logger *log.Logger
}

func main() {
	fullScrapeFlag := flag.Bool("full", false, "Run a full wiki enhancement for all events")
	cronFlag := flag.Bool("cron", false, "Run as a cron job service")
	flag.Parse()

	// Set up logging to both file and console
	logFile, err := os.Create("wiki_events_log.txt")
	if err != nil {
		fmt.Printf("Failed to create log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Create a multi-writer to write logs to both file and console
	multi := io.MultiWriter(logFile, os.Stdout)
	logger := log.New(multi, "WIKI-SCRAPER: ", log.LstdFlags)
	
	logger.Println("🚀 Starting Wiki Event Scraper")

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		logger.Printf("Warning: .env file not found: %v", err)
	}

	// Load configuration
	if err := config.LoadConfig("config/config.json"); err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		logger.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()

	app := &ScraperApp{
		db:     db,
		logger: logger,
	}

	if *fullScrapeFlag {
		logger.Println("Running full wiki enhancement for all events")
		app.runFullEnhancement()
		return
	} else if *cronFlag {
		logger.Println("Starting cron service mode")
		app.runCronService()
		return
	} else {
		logger.Println("Running recent event wiki enhancement (future + last 2 months)")
		app.runRecentEnhancement()
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

	// Optimize connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func (app *ScraperApp) runFullEnhancement() {
	startTime := time.Now()
	app.logger.Println("Starting full event enhancement with Wikipedia data...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	// Create Wiki event scraper
	wikiScraper := scrapers.NewWikiEventScraper(app.db)

	// Enhance all events with Wikipedia data
	updatedCount, err := wikiScraper.EnhanceEventsWithWikiData(ctx)
	if err != nil {
		app.logger.Fatalf("Failed to enhance events with Wikipedia data: %v", err)
	}

	app.logger.Printf("🏁 Successfully enhanced %d events with Wikipedia data in %v!",
		updatedCount, time.Since(startTime).Round(time.Second))
}

func (app *ScraperApp) runRecentEnhancement() {
    startTime := time.Now()
    app.logger.Println("Starting recent event enhancement with Wikipedia data...")

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
    defer cancel()

    // Calculate cutoff date - last 2 months
    twoMonthsAgo := time.Now().AddDate(0, -2, 0)
    app.logger.Printf("Using cutoff date: %s", twoMonthsAgo.Format("2006-01-02"))

    // Create Wiki event scraper
    wikiScraper := scrapers.NewWikiEventScraper(app.db)

    // Use the new function that only enhances recent events
    updatedCount, err := wikiScraper.EnhanceRecentEventsWithWikiData(ctx, twoMonthsAgo)
    if err != nil {
        app.logger.Fatalf("Failed to enhance recent events with Wikipedia data: %v", err)
    }

    app.logger.Printf("🏁 Successfully enhanced %d recent events with Wikipedia data in %v!",
        updatedCount, time.Since(startTime).Round(time.Second))
}

func (app *ScraperApp) runCronService() {
	app.logger.Println("Starting cron service for wiki event enhancements")

	c := cron.New(
		cron.WithLogger(cron.VerbosePrintfLogger(app.logger)),
		cron.WithLocation(time.UTC),
	)

	// Run recent enhancement weekly on Monday at 04:00 UTC
	if _, err := c.AddFunc("0 4 * * 1", app.runRecentEnhancement); err != nil {
		app.logger.Fatalf("Failed to add weekly wiki enhancement job: %v", err)
	}
	
	// Run full enhancement monthly on the 2nd at 05:00 UTC
	if _, err := c.AddFunc("0 5 2 * *", app.runFullEnhancement); err != nil {
		app.logger.Fatalf("Failed to add monthly full enhancement job: %v", err)
	}

	c.Start()
	app.logger.Println("Cron scheduler started with the following schedule:")
	app.logger.Println("- Weekly recent event enhancement every Monday at 04:00 UTC")
	app.logger.Println("- Monthly full event enhancement on the 2nd at 05:00 UTC")

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	app.logger.Println("Shutting down wiki scraper service...")
	
	c.Stop()
	
	if err := app.db.Close(); err != nil {
		app.logger.Printf("Error closing database connection: %v", err)
	}
	
	app.logger.Println("Wiki scraper service stopped gracefully")
}