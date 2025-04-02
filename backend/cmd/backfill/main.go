package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
	
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"mma-scheduler/config"
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

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	
	// Get fighter IDs - use direct string values to avoid additional queries
	volkID := "bde47112-3e2c-42b9-8a03-cf87bbbc1db7" // Alexander Volkanovski
	lopesID := "c88fd3e5-3b14-4687-863e-fac05aadf612" // Diego Lopes
	hollowayID := "34af9153-c95f-4310-80d2-e616ac1e3537" // Max Holloway
	ortegaID := "eb3b02c8-0a35-4d3f-a9af-a7a2f05fa97a" // Brian Ortega
	zombieID := "e14f4cf4-4166-477d-b772-8f2f55064b35" // Korean Zombie
	makhachevID := "ca030834-2c31-4934-8db9-ddd2f6a168a2" // Islam Makhachev
	rodriguezID := "7e9d0341-ec78-434a-95e8-9de1e384df60" // Yair Rodriguez
	evloevID := "a3efafcf-d4b9-46af-bde4-af1fc0edfe6d" // Movsar Evloev
	
	// Set up missing fights
	missingFights := []struct {
		eventName      string
		fighter1ID     string
		fighter1Name   string
		fighter2ID     string
		fighter2Name   string
		weightClass    string
		isMainEvent    bool
		isTitleFight   bool
		fighter1Rank   string
		fighter2Rank   string
		winner         string
	}{
		// Volkanovski UFC 290 vs Yair
		{
			eventName:     "UFC 290",
			fighter1ID:    volkID,
			fighter1Name:  "Alexander Volkanovski",
			fighter2ID:    rodriguezID,
			fighter2Name:  "Yair Rodriguez",
			weightClass:   "Featherweight Title Bout",
			isMainEvent:   true,
			isTitleFight:  true,
			fighter1Rank:  "C",
			fighter2Rank:  "IC",
			winner:        volkID,
		},
		// Volkanovski UFC 284 vs Makhachev
		{
			eventName:     "UFC 284",
			fighter1ID:    makhachevID,
			fighter1Name:  "Islam Makhachev",
			fighter2ID:    volkID,
			fighter2Name:  "Alexander Volkanovski",
			weightClass:   "Lightweight Title Bout",
			isMainEvent:   true,
			isTitleFight:  true,
			fighter1Rank:  "C",
			fighter2Rank:  "C-FW",
			winner:        makhachevID,
		},
		// Volkanovski UFC 276 vs Holloway 3
		{
			eventName:     "UFC 276",
			fighter1ID:    volkID,
			fighter1Name:  "Alexander Volkanovski",
			fighter2ID:    hollowayID,
			fighter2Name:  "Max Holloway",
			weightClass:   "Featherweight Title Bout",
			isMainEvent:   false,
			isTitleFight:  true,
			fighter1Rank:  "C",
			fighter2Rank:  "1",
			winner:        volkID,
		},
		// Volkanovski UFC 273 vs Korean Zombie
		{
			eventName:     "UFC 273",
			fighter1ID:    volkID,
			fighter1Name:  "Alexander Volkanovski",
			fighter2ID:    zombieID,
			fighter2Name:  "Chan Sung Jung",
			weightClass:   "Featherweight Title Bout",
			isMainEvent:   true,
			isTitleFight:  true,
			fighter1Rank:  "C",
			fighter2Rank:  "4",
			winner:        volkID,
		},
		// Volkanovski UFC 266 vs Ortega
		{
			eventName:     "UFC 266",
			fighter1ID:    volkID,
			fighter1Name:  "Alexander Volkanovski",
			fighter2ID:    ortegaID,
			fighter2Name:  "Brian Ortega",
			weightClass:   "Featherweight Title Bout",
			isMainEvent:   true,
			isTitleFight:  true,
			fighter1Rank:  "C",
			fighter2Rank:  "2",
			winner:        volkID,
		},
		// Diego Lopes UFC 288 vs Evloev
		{
			eventName:     "UFC 288",
			fighter1ID:    evloevID,
			fighter1Name:  "Movsar Evloev",
			fighter2ID:    lopesID,
			fighter2Name:  "Diego Lopes",
			weightClass:   "Featherweight Bout",
			isMainEvent:   false,
			isTitleFight:  false,
			fighter1Rank:  "10",
			fighter2Rank:  "",
			winner:        evloevID,
		},
	}
	
	for _, fight := range missingFights {
		// Get event ID
		var eventID string
		eventQuery := ""
		
		if fight.eventName == "UFC 290" {
			eventQuery = "SELECT id FROM events WHERE name LIKE 'UFC 290%'"
		} else if fight.eventName == "UFC 284" {
			eventQuery = "SELECT id FROM events WHERE name LIKE 'UFC 284%'"
		} else if fight.eventName == "UFC 276" {
			eventQuery = "SELECT id FROM events WHERE name LIKE 'UFC 276%'"
		} else if fight.eventName == "UFC 273" {
			eventQuery = "SELECT id FROM events WHERE name LIKE 'UFC 273%'"
		} else if fight.eventName == "UFC 266" {
			eventQuery = "SELECT id FROM events WHERE name LIKE 'UFC 266%'"
		} else if fight.eventName == "UFC 288" {
			eventQuery = "SELECT id FROM events WHERE name LIKE 'UFC 288%'"
		}
		
		err := db.QueryRow(eventQuery).Scan(&eventID)
		if err != nil {
			log.Printf("Warning: Could not find event ID for %s: %v", fight.eventName, err)
			continue
		}
		
		log.Printf("Found event ID: %s for event %s", eventID, fight.eventName)
		
		// Determine winner
		var winnerID *string
		if fight.winner != "" {
			winnerID = &fight.winner
		}
		
		// Insert the fight
		_, err = db.Exec(`
			INSERT INTO fights (
				event_id, fighter1_id, fighter2_id, fighter1_name, fighter2_name, 
				weight_class, is_main_event, was_title_fight,
				fighter1_rank, fighter2_rank, winner_id,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (event_id, fighter1_id, fighter2_id) DO UPDATE SET
				fighter1_name = EXCLUDED.fighter1_name,
				fighter2_name = EXCLUDED.fighter2_name,
				weight_class = EXCLUDED.weight_class,
				is_main_event = EXCLUDED.is_main_event,
				was_title_fight = EXCLUDED.was_title_fight,
				fighter1_rank = EXCLUDED.fighter1_rank,
				fighter2_rank = EXCLUDED.fighter2_rank,
				winner_id = EXCLUDED.winner_id,
				updated_at = EXCLUDED.updated_at
		`, 
			eventID, fight.fighter1ID, fight.fighter2ID, fight.fighter1Name, fight.fighter2Name,
			fight.weightClass, fight.isMainEvent, fight.isTitleFight,
			fight.fighter1Rank, fight.fighter2Rank, winnerID,
			time.Now(), time.Now(),
		)
		
		if err != nil {
			log.Printf("Failed to save fight %s vs %s: %v", fight.fighter1Name, fight.fighter2Name, err)
		} else {
			log.Printf("Successfully saved fight: %s vs %s at event %s", 
				fight.fighter1Name, fight.fighter2Name, fight.eventName)
		}
	}
	
	log.Printf("Backfill complete! Volkanovski should now have 9 fights and Lopes should have 6 fights.")
}