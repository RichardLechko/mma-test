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
		// Silently continue if .env not found
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

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	tournamentScraper := scrapers.NewTournamentScraper(db)

	tournamentEvents := []string{
		"UFC 1: The Beginning",
		"UFC 2: No Way Out",
		"UFC 3: The American Dream",
		"UFC 4: Revenge of the Warriors",
		"UFC 5: The Return of the Beast",
		"UFC 6: Clash of the Titans",
		"UFC 7: The Brawl in Buffalo",
		"UFC: The Ultimate Ultimate",
		"UFC 8: David vs. Goliath",
		"UFC 10: The Tournament",
		"UFC 11: The Proving Ground",
		"UFC: The Ultimate Ultimate 2",
		"UFC 12: Judgement Day",
		"UFC 13: The Ultimate Force",
		"UFC 14: Showdown",
		"UFC 15: Collision Course",
		"UFC Japan: Ultimate Japan",
		"UFC 16: Battle in the Bayou",
		"UFC 17: Redemption",
		"UFC 23: Ultimate Japan 2",
	}
	
	// Create a map from event names to event IDs
	eventMap := make(map[string]string)
	
	// Query to get event IDs for the tournament events
	rows, err := db.QueryContext(ctx, `
		SELECT id, name FROM events 
		WHERE name = ANY($1)
	`, tournamentEvents)
	
	if err != nil {
		log.Fatalf("Error querying events: %v", err)
	}
	defer rows.Close()
	
	// Populate the event map
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			log.Printf("Error scanning event: %v", err)
			continue
		}
		eventMap[name] = id
	}
	
	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating event rows: %v", err)
	}

	// Process each tournament event
	for _, eventName := range tournamentEvents {
		eventID, exists := eventMap[eventName]
		if !exists {
			log.Printf("Event '%s' not found in database, skipping", eventName)
			continue
		}
		
		// Process the tournament event
		log.Printf("Processing tournament event: %s", eventName)
		err := tournamentScraper.ProcessTournament(ctx, eventID, eventName)
		if err != nil {
			log.Printf("Error processing tournament %s: %v", eventName, err)
			continue
		}
	}

	log.Printf("Tournament processing completed!")
}