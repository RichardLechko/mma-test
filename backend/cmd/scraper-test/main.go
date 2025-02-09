package main

import (
    "context"
    "fmt"
    "log"
    "time"
    "database/sql"
    
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
    scraper := scrapers.NewFighterScraper(config)

    
    log.Println("Starting to scrape fighters...")
    fighters, err := scraper.ScrapeFighters()
    if err != nil {
        log.Fatalf("Error scraping fighters: %v", err)
    }

    log.Printf("Successfully scraped %d fighters\n", len(fighters))
    for _, fighter := range fighters {
        fmt.Printf("\nFighter Details:\n")
        fmt.Printf("Name: %s\n", fighter.Name)
        if fighter.Nickname != "" {
            fmt.Printf("Nickname: %s\n", fighter.Nickname)
        }
        fmt.Printf("Weight Class: %s\n", fighter.WeightClass)
        fmt.Printf("Record: %d-%d-%d\n", fighter.Record.Wins, fighter.Record.Losses, fighter.Record.Draws)
        if fighter.Rank != "" {
            fmt.Printf("Rank: %s\n", fighter.Rank)
        }
        fmt.Printf("Status: %s\n", fighter.Status)
        fmt.Println("----------------------------------------")
    }

    fmt.Println("\nDo you want to proceed with saving these fighters to the database? (y/n)")
    var response string
    fmt.Scanln(&response)
    if response != "y" {
        log.Println("Database operations skipped.")
        return
    }

    
    dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer dbCancel()

    for _, fighter := range fighters {
        fmt.Printf("\nProcessing fighter: %s\n", fighter.Name)
    
        query := `
            INSERT INTO fighters (
                ufc_id, name, nickname, weight_class, rank, status,
                wins, losses, draws, ko_wins, sub_wins, first_round,
                created_at, updated_at
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
            )
            ON CONFLICT (ufc_id) 
            DO UPDATE SET
                name = EXCLUDED.name,
                nickname = EXCLUDED.nickname,
                weight_class = EXCLUDED.weight_class,
                rank = EXCLUDED.rank,
                status = EXCLUDED.status,
                wins = EXCLUDED.wins,
                losses = EXCLUDED.losses,
                draws = EXCLUDED.draws,
                ko_wins = EXCLUDED.ko_wins,
                sub_wins = EXCLUDED.sub_wins,
                first_round = EXCLUDED.first_round,
                updated_at = EXCLUDED.updated_at
            RETURNING id`
    
        now := time.Now()
        var fighterID string

        err = db.QueryRowContext(dbCtx, query,
            fighter.UFCID,
            fighter.Name,
            fighter.Nickname,
            fighter.WeightClass,
            fighter.Rank,
            fighter.Status,
            fighter.Record.Wins,
            fighter.Record.Losses,
            fighter.Record.Draws,
            fighter.Record.KOWins,
            fighter.Record.SubWins,
            fighter.FirstRound,
            now,
            now,
        ).Scan(&fighterID)

        if err != nil {
            log.Printf("Warning: Failed to save fighter %s: %v", fighter.Name, err)
            log.Printf("Failed fighter data: %+v", fighter)
            continue
        }

        fmt.Printf("Successfully saved/updated fighter with ID: %s\n", fighterID)
        fmt.Println("----------------------------------------")
    }

    log.Println("Fighter scraping and database update completed!")
}