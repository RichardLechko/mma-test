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
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Create the scraper config
	scraperConfig := &scrapers.ScraperConfig{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}

	// Create the fighter scraper
	fighterScraper := scrapers.NewWikiFighterScraper(scraperConfig)

	// Get all active fighters from the database
	fighters, err := getActiveFighters(db)
	if err != nil {
		log.Fatalf("Error getting fighters: %v", err)
	}

	log.Printf("Found %d active fighters to process", len(fighters))

	// Create database context with longer timeout
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer dbCancel()

	log.Printf("Starting to scrape extra info for %d active fighters...", len(fighters))

	// Add a processing counter
	processed := 0
	startTime := time.Now()
	
	// Process fighters with a time limit for the entire operation
	for _, fighter := range fighters {
		// Create a timeout for each individual fighter processing
		fighterCtx, fighterCancel := context.WithTimeout(context.Background(), 30*time.Second)
		
		// Use a channel to handle the processing with timeout
		done := make(chan bool)
		var info *scrapers.FighterExtraInfo
		var scrapeErr error
		
		go func() {
			info, scrapeErr = fighterScraper.ScrapeExtraInfo(fighter.Name, fighter.WikiURL, "", fighter.Wins, fighter.Losses)
			done <- true
		}()
		
		// Wait for either completion or timeout
		select {
		case <-done:
			// Processing completed normally
		case <-fighterCtx.Done():
			log.Printf("%s: ⚠️ Processing timed out after 30 seconds", fighter.Name)
			fighterCancel()
			continue
		}
		
		fighterCancel()
		
		if scrapeErr != nil {
			log.Printf("%s: ❌ Failed - %v", fighter.Name, scrapeErr)
			continue
		}

		if info == nil {
			log.Printf("%s: ❌ No data found", fighter.Name)
			continue
		}

		// UPDATING FIGHTER IN DATABASE
		log.Printf("UPDATING FIGHTER with ID %s", fighter.ID)

		// Update win methods if appropriate
		shouldUpdateWinMethods := false
		scrapedWinMethods := info.KOWins + info.SubWins + info.DecWins

		log.Printf("  - Win methods: KO:%d, Sub:%d, Dec:%d (total: %d)",
			info.KOWins, info.SubWins, info.DecWins, scrapedWinMethods)

		// Case 1: Fighter has UFC wins, and scraped methods match the total
		if fighter.Wins > 0 && scrapedWinMethods == fighter.Wins {
			shouldUpdateWinMethods = true
		}

		// Only update if appropriate
		if shouldUpdateWinMethods {
			_, err = db.ExecContext(dbCtx, `
				UPDATE fighters SET 
					ko_wins = $1, 
					sub_wins = $2, 
					dec_wins = $3, 
					updated_at = NOW()
				WHERE id = $4
			`, info.KOWins, info.SubWins, info.DecWins, fighter.ID)

			if err != nil {
				log.Printf("Failed to update win methods: %v", err)
			} else {
				log.Printf("%s: Win methods updated", fighter.Name)
			}
		} else {
			log.Printf("  - SKIPPING win method update")
		}

		// Update loss methods if appropriate
		shouldUpdateLossMethods := false
		scrapedLossMethods := info.KOLosses + info.SubLosses + info.DecLosses + info.DQLosses

		log.Printf("  - Loss methods: KO:%d, Sub:%d, Dec:%d, DQ:%d (total: %d)",
			info.KOLosses, info.SubLosses, info.DecLosses, info.DQLosses, scrapedLossMethods)

		// Case 1: Fighter has UFC losses, and scraped methods match the total
		if fighter.Losses > 0 && scrapedLossMethods == fighter.Losses {
			shouldUpdateLossMethods = true
		}

		// Only update if appropriate
		if shouldUpdateLossMethods {
			_, err = db.ExecContext(dbCtx, `
				UPDATE fighters SET 
					loss_by_ko = $1, 
					loss_by_sub = $2, 
					loss_by_dec = $3, 
					loss_by_dq = $4,
					updated_at = NOW()
				WHERE id = $5
			`, info.KOLosses, info.SubLosses, info.DecLosses, info.DQLosses, fighter.ID)

			if err != nil {
				log.Printf("Failed to update loss methods: %v", err)
			} else {
				log.Printf("%s: Loss methods updated", fighter.Name)
			}
		} else {
			log.Printf("  - SKIPPING loss method update")
		}

		log.Printf("  - Fighting out of: %s", info.FightingOutOf)

		if info.FightingOutOf != "" {
			_, err = db.ExecContext(dbCtx, `
				UPDATE fighters SET 
					fighting_out_of = $1,
					updated_at = NOW()
				WHERE id = $2
			`, info.FightingOutOf, fighter.ID)

			if err != nil {
				log.Printf("Failed to update fighter location info: %v", err)
			} else {
				log.Printf("%s: Fighting location updated", fighter.Name)
			}
		}
		
		// Track progress
		processed++
		elapsedTime := time.Since(startTime)
		timePerFighter := elapsedTime / time.Duration(processed)
		log.Printf("%s: ✅ Processing completed (%d/%d, ~%v per fighter)", 
			fighter.Name, processed, len(fighters), timePerFighter.Round(time.Second))
	}
	
	log.Printf("🏁 Processing completed for %d fighters in %v", processed, time.Since(startTime).Round(time.Second))
}

type Fighter struct {
	ID         string
	Name       string
	WikiURL    string
	Wins       int
	Losses     int
	KOWins     int
	SubWins    int
	DecWins    int
	NoContests int
	Status     *string 
}

// Get all active fighters from the database
func getActiveFighters(db *sql.DB) ([]Fighter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query all active fighters
	rows, err := db.QueryContext(ctx, `
		SELECT 
			id, name, COALESCE(wiki_url, ''), wins, losses, 
			COALESCE(ko_wins, 0), COALESCE(sub_wins, 0), COALESCE(dec_wins, 0), 
			status
		FROM fighters
		ORDER BY RANDOM()
	`)
	if err != nil {
		return nil, fmt.Errorf("error querying fighters: %v", err)
	}
	defer rows.Close()

	var fighters []Fighter

	for rows.Next() {
		var f Fighter
		if err := rows.Scan(&f.ID, &f.Name, &f.WikiURL, &f.Wins, &f.Losses,
			&f.KOWins, &f.SubWins, &f.DecWins, &f.Status); err != nil {
			return nil, fmt.Errorf("error scanning fighter: %v", err)
		}
		fighters = append(fighters, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fighters: %v", err)
	}

	return fighters, nil
}