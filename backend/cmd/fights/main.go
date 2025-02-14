package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"mma-scheduler/config"
	"mma-scheduler/internal/models"
	"mma-scheduler/pkg/scrapers"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	if err := config.LoadConfig("config/config.json"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db := setupDatabase()
	defer db.Close()

	events := fetchEvents(db)
	processEvents(db, events)
}

func setupDatabase() *sql.DB {
	dbConfig := config.GetDatabaseConfig()
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=require&pool_max_conns=%d&pool_min_conns=%d",
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	return db
}

func fetchEvents(db *sql.DB) []struct {
	ID        string
	Name      string
	EventDate time.Time
} {
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

		if isBeforeUFC229(event.Name) {
			log.Printf("Reached UFC 229 cutoff point. Stopping scraper.")
			break
		}

		events = append(events, event)
	}

	return events
}

func processEvents(db *sql.DB, events []struct {
	ID        string
	Name      string
	EventDate time.Time
}) {
	log.Printf("Starting to scrape %d events...\n", len(events))

	scraper := scrapers.NewFightScraper()
	rateLimiter := time.NewTicker(2 * time.Second)
	defer rateLimiter.Stop()

	for _, event := range events {
		<-rateLimiter.C

		fights, _, err := scraper.ScrapeFights(event.Name, event.EventDate)
		if err != nil {
			log.Printf("\nERROR: Failed to scrape %s: %v", event.Name, err)
			if strings.Contains(err.Error(), "already processed") {
				continue
			}
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("✓ %s (Found %d fights)", event.Name, len(fights))
		
		if err := saveFights(db, event.ID, fights); err != nil {
			log.Printf("Error saving fights for %s: %v", event.Name, err)
			continue
		}

		// Add delay every 20 successful scrapes
		if event.EventDate.After(time.Now()) {
			time.Sleep(10 * time.Second)
		}
	}

	log.Println("Fight scraping completed!")
}

func saveFights(db *sql.DB, eventID string, fights []models.Fight) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	for _, fight := range fights {
		fighter1ID, err := ensureFighter(tx, fight.Fighter1)
		if err != nil {
			return fmt.Errorf("error ensuring fighter1 %s: %v", fight.Fighter1, err)
		}

		fighter2ID, err := ensureFighter(tx, fight.Fighter2)
		if err != nil {
			return fmt.Errorf("error ensuring fighter2 %s: %v", fight.Fighter2, err)
		}

		if err := insertFight(tx, eventID, fighter1ID, fighter2ID, fight); err != nil {
			return fmt.Errorf("error inserting fight: %v", err)
		}
	}

	return tx.Commit()
}

func insertFight(tx *sql.Tx, eventID, fighter1ID, fighter2ID string, fight models.Fight) error {
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
	err := tx.QueryRow(
		query,
		eventID,
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

	return err
}

func ensureFighter(tx *sql.Tx, name string) (string, error) {
	name = strings.TrimSpace(name)

	var id string
	err := tx.QueryRow("SELECT id FROM fighters WHERE name = $1", name).Scan(&id)
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

	return id, nil
}

func isBeforeUFC229(eventName string) bool {
	re := regexp.MustCompile(`UFC (\d+)`)
	matches := re.FindStringSubmatch(eventName)
	if len(matches) > 1 {
		if number, err := strconv.Atoi(matches[1]); err == nil {
			return number < 229
		}
	}
	return false
}