package main

import (
    "context"
    "fmt"
    "log"
    "time"
    "database/sql"
    "strings"
    
    _ "github.com/jackc/pgx/v5/stdlib"
    "mma-scheduler/pkg/scrapers"
    "mma-scheduler/config"
    "github.com/joho/godotenv"
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

    
    log.Printf("Database Host: %s", dbConfig.Host)
    log.Printf("Database User: %s", dbConfig.User)
    log.Printf("Database Name: %s", dbConfig.Database)
    log.Printf("Database Port: %d", dbConfig.Port)
    
    db, err := sql.Open("pgx", connStr)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := db.PingContext(ctx); err != nil {
        log.Printf("Connection error details: %+v", err)
        log.Fatalf("Failed to ping database: %v", err)
    }
    log.Println("Successfully connected to database")

    
    config := scrapers.DefaultConfig()
    scraper := scrapers.NewEventScraper(config)

    
    log.Println("Starting to scrape events...")
    events, err := scraper.ScrapeEvents()
    if err != nil {
        log.Fatalf("Error scraping events: %v", err)
    }
    
    dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer dbCancel()

    for _, event := range events {
        fmt.Printf("\nProcessing event: %s\n", event.Name)
    
        query := `
            INSERT INTO events (
                name, event_date, venue, city, country, status, 
                created_at, updated_at
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8
            )
            ON CONFLICT (name) 
            DO UPDATE SET
                event_date = EXCLUDED.event_date,
                venue = EXCLUDED.venue,
                city = EXCLUDED.city,
                country = EXCLUDED.country,
                updated_at = EXCLUDED.updated_at
            RETURNING id`
    
        now := time.Now()
        var eventID string
    
        eventDate, err := time.Parse("2006-01-02 15:04:05 -0700 MST", event.Date)
if err != nil {
    log.Printf("Warning: Failed to parse date for event %s: %v (date: %s)", event.Name, err, event.Date)
    continue
}
    
        
        var venue, city, country string
        if event.Location != "" {
            locationParts := strings.Split(event.Location, ", ")
            switch len(locationParts) {
                case 3:
                    venue = locationParts[0]
                    city = locationParts[1]
                    country = locationParts[2]
                case 2:
                    city = locationParts[0]
                    country = locationParts[1]
                case 1:
                    city = locationParts[0]
            }
        }

        err = db.QueryRowContext(dbCtx, query,
            event.Name,
            eventDate,
            venue,
            city,
            country,
            "announced", 
            now,        
            now,        
        ).Scan(&eventID)

        if err != nil {
            log.Printf("Warning: Failed to save event %s: %v", event.Name, err)
            log.Printf("Failed event data: %+v", event)
            continue
        }
    }

    log.Println("Scraping and database update completed!")
}