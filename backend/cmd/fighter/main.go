package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
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
		log.Printf("Connection error details: %+v", err)
		log.Fatalf("Failed to ping database: %v", err)
	}

	
	rows, err := db.Query(`
        SELECT id, name, event_date AT TIME ZONE 'UTC' as event_date
        FROM events 
        WHERE status = 'announced'
        ORDER BY event_date DESC
    `)
	if err != nil {
		log.Fatalf("Failed to fetch events: %v", err)
	}
	defer rows.Close()

	var events []struct {
		ID        string
		Name      string
		EventDate time.Time
	}

	for rows.Next() {
		var event struct {
			ID        string
			Name      string
			EventDate time.Time
		}
		if err := rows.Scan(&event.ID, &event.Name, &event.EventDate); err != nil {
			log.Printf("Error scanning event row: %v", err)
			continue
		}
		events = append(events, event)
	}

	log.Printf("Starting to scrape %d events...\n", len(events))

	
	scraper := scrapers.NewFightScraper()

	
	rateLimiter := time.NewTicker(2 * time.Second)
	defer rateLimiter.Stop()

	successCount := 0
	errorCount := 0

	for _, event := range events {
		<-rateLimiter.C

		fights, urls, err := scraper.ScrapeFights(event.Name, event.EventDate)
		if err != nil {
			log.Printf("\nERROR: Failed to scrape %s: %v\n", event.Name, err)
			errorCount++

			if errorCount > 5 {
				log.Println("Too many errors, increasing delay...")
				time.Sleep(30 * time.Second)
				errorCount = 0
			}
			continue
		}

		if len(fights) == 0 {
			log.Printf("\nWARNING: No fights found for event: %s\n", event.Name)
			log.Printf("Attempted URLs:\n")
			for _, url := range urls {
				log.Printf("- %s\n", url)
			}
			continue
		}

		fmt.Printf("✓ %s\n", event.Name) 

		
		for _, fight := range fights {
			
			fighter1ID, err := ensureFighter(db, fight.Fighter1)
			if err != nil {
				log.Printf("Error ensuring fighter1 %s: %v", fight.Fighter1, err)
				continue
			}

			fighter2ID, err := ensureFighter(db, fight.Fighter2)
			if err != nil {
				log.Printf("Error ensuring fighter2 %s: %v", fight.Fighter2, err)
				continue
			}

			
			query := `
                INSERT INTO fights (
                    event_id, fighter1_id, fighter2_id, 
                    fighter1_name, fighter2_name,
                    weight_class, is_main_event, fight_order,
                    created_at, updated_at
                ) VALUES (
                    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
                )
                ON CONFLICT ON CONSTRAINT uq_fights_event_fighters 
                DO UPDATE SET
                    weight_class = EXCLUDED.weight_class,
                    is_main_event = EXCLUDED.is_main_event,
                    fight_order = EXCLUDED.fight_order,
                    updated_at = EXCLUDED.updated_at
                RETURNING id`

			var fightID string
			err = db.QueryRow(
				query,
				event.ID,
				fighter1ID,
				fighter2ID,
				fight.Fighter1,
				fight.Fighter2,
				fight.WeightClass,
				fight.IsMainEvent,
				fight.Order,
				time.Now(),
				time.Now(),
			).Scan(&fightID)

			if err != nil {
				log.Printf("Error saving fight: %v", err)
				continue
			}
		}

		successCount++
		if successCount%20 == 0 {
			time.Sleep(30 * time.Second)
		}
	}

	log.Println("Fight scraping completed!")
}

func ensureFighter(db *sql.DB, name string) (string, error) {
	
	name = strings.TrimSpace(name)

	
	var id string
	err := db.QueryRow("SELECT id FROM fighters WHERE name = $1", name).Scan(&id)
	if err == nil {
		return id, nil
	}

	if err != sql.ErrNoRows {
		return "", fmt.Errorf("error checking fighter existence: %v", err)
	}

	
	tempID := strings.ToLower(name)
	tempID = strings.ReplaceAll(tempID, " ", "-")
	tempID = strings.ReplaceAll(tempID, "'", "")
	tempID = strings.ReplaceAll(tempID, ".", "")
	tempID = fmt.Sprintf("temp-%s-%s", tempID, time.Now().Format("20060102"))

	
	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	err = tx.QueryRow(`
        INSERT INTO fighters (
            name,
            ufc_id,
            status,
            created_at,
            updated_at
        )
        VALUES ($1, $2, $3, NOW(), NOW())
        RETURNING id
    `, name, tempID, "pending").Scan(&id)

	if err != nil {
		return "", fmt.Errorf("error creating placeholder fighter: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("error committing transaction: %v", err)
	}

	return id, nil
}
