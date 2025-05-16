// main package provides a command-line tool to enhance events with Wikipedia data
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

type WikiEnhancerApp struct {
    db             *sql.DB
    logger         *log.Logger
    wikiScraper    *scrapers.WikiEventScraper
}

func main() {
    // Define flags similar to the main scraper
    fullScrapeFlagPtr := flag.Bool("full", false, "Run a full scrape without time limitations")
    cronFlagPtr := flag.Bool("cron", false, "Run as a cron job service")
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
    logger := log.New(multi, "WIKI-ENHANCER: ", log.LstdFlags)
    logger.Println("=== Starting Wiki Event Enhancement ===")
    logger.Printf("Time: %s", time.Now().Format(time.RFC3339))
    
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
    
    app := &WikiEnhancerApp{
        db:          db,
        logger:      logger,
        wikiScraper: scrapers.NewWikiEventScraper(db),
    }

    if *fullScrapeFlagPtr {
        logger.Println("Running full wiki enhancement mode")
        app.runFullWikiEnhancement()
        return
    } else if *cronFlagPtr {
        logger.Println("Starting cron service mode")
        app.runCronService()
        return
    } else {
        logger.Println("Running recent wiki enhancement (last 2 months)")
        app.runRecentWikiEnhancement()
        return
    }
}

func connectToDatabase() (*sql.DB, error) {
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
        return nil, fmt.Errorf("Failed to open database: %w", err)
    }
    
    db.SetMaxOpenConns(dbConfig.MaxOpenConns)
    db.SetMaxIdleConns(dbConfig.MaxIdleConns)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(2 * time.Minute)
    
    // Ping database to verify connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := db.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("Failed to ping database: %w", err)
    }
    
    return db, nil
}

func (app *WikiEnhancerApp) runFullWikiEnhancement() {
    startTime := time.Now()
    app.logger.Println("Starting full wiki enhancement for all events")
    
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
    defer cancel()
    
    // Enhance all events with Wikipedia data
    updatedCount, err := app.wikiScraper.EnhanceEventsWithWikiData(ctx)
    if err != nil {
        app.logger.Fatalf("Failed to enhance events with Wikipedia data: %v", err)
    }
    
    app.logger.Printf("🏁 Successfully enhanced %d events with Wikipedia data! Completed in %v", 
        updatedCount, time.Since(startTime).Round(time.Second))
}

func (app *WikiEnhancerApp) runRecentWikiEnhancement() {
    startTime := time.Now()
    app.logger.Println("Starting wiki enhancement for recent events (last 2 months)")
    
    twoMonthsAgo := time.Now().AddDate(0, -2, 0)
    app.logger.Printf("Using cutoff date: %s", twoMonthsAgo.Format("2006-01-02"))
    
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
    defer cancel()
    
    updatedCount, err := app.wikiScraper.EnhanceRecentEventsWithWikiData(ctx, twoMonthsAgo)
    if err != nil {
        app.logger.Fatalf("Failed to enhance recent events with Wikipedia data: %v", err)
    }
    
    app.logger.Printf("🏁 Successfully enhanced recent events with Wikipedia data! Updated %d events in %v", 
        updatedCount, time.Since(startTime).Round(time.Second))
}

func (app *WikiEnhancerApp) runCronService() {
    app.logger.Println("Starting cron service for wiki enhancements")

    c := cron.New(
        cron.WithLogger(cron.VerbosePrintfLogger(app.logger)),
        cron.WithLocation(time.UTC),
    )

    // Add weekly job for recent events - run every Wednesday at 04:00 UTC
    if _, err := c.AddFunc("0 4 * * 3", app.runRecentWikiEnhancement); err != nil {
        app.logger.Fatalf("Failed to add weekly update job: %v", err)
    }
    
    // Add monthly job for full enhancement - run on the 2nd of each month at 04:00 UTC
    if _, err := c.AddFunc("0 4 2 * *", app.runFullWikiEnhancement); err != nil {
        app.logger.Fatalf("Failed to add monthly full update job: %v", err)
    }

    c.Start()
    app.logger.Println("Cron scheduler started with the following schedule:")
    app.logger.Println("- Weekly wiki updates every Wednesday at 04:00 UTC")
    app.logger.Println("- Monthly full wiki enhancement on the 2nd at 04:00 UTC")

    // Wait for termination signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
    <-quit

    app.logger.Println("Shutting down wiki enhancer service...")
    
    c.Stop()
    
    if err := app.db.Close(); err != nil {
        app.logger.Printf("Error closing database connection: %v", err)
    }
    
    app.logger.Println("Wiki enhancer service stopped gracefully")
}