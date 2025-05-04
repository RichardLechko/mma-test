// main package provides a command-line tool to enhance events with Wikipedia data
package main

import (
    "context"
    "database/sql"
    "fmt"
    "io"
    "log"
    "os"
    "time"
    "github.com/joho/godotenv"
    _ "github.com/jackc/pgx/v5/stdlib"
    "mma-scheduler/config"
    "mma-scheduler/pkg/scrapers"
)

func main() {
    // Set up logging to both file and console
    logFile, err := os.Create("wiki_events_log.txt")
    if err != nil {
        fmt.Printf("Failed to create log file: %v\n", err)
        os.Exit(1)
    }
    defer logFile.Close()
   
    // Create a multi-writer to write logs to both file and console
    multi := io.MultiWriter(logFile, os.Stdout)
    log.SetOutput(multi)
   
    log.Println("=== Starting Wiki Event Enhancement ===")
    log.Printf("Time: %s", time.Now().Format(time.RFC3339))
    
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
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
    defer cancel()
    
    if err := db.PingContext(ctx); err != nil {
        log.Fatalf("Failed to ping database: %v", err)
    }
    log.Println("Connected to database")
    
    // Create Wiki event scraper
    wikiScraper := scrapers.NewWikiEventScraper(db)
    
    // Start scraping and enhancing events
    log.Println("Starting to enhance events with Wikipedia data...")
    
    updatedCount, err := wikiScraper.EnhanceEventsWithWikiData(ctx)
    if err != nil {
        log.Fatalf("Failed to enhance events with Wikipedia data: %v", err)
    }
    
    log.Printf("Successfully enhanced %d events with Wikipedia data!", updatedCount)
}