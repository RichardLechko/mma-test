package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"
)

// Fighter represents a fighter record from the database
type Fighter struct {
	ID            string
	Name          string
	WikiURL       string
	Wins          int
	Losses        int
	KOWins        int
	SubWins       int
	DecWins       int
	LossByKO      int
	LossBySub     int
	LossByDec     int
	LossByDQ      int
	NoContests    int
	FightingOutOf string
}

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

	// Get all fighters from the database
	fighters, err := getAllFighters(db)
	if err != nil {
		log.Fatalf("Error getting fighters: %v", err)
	}

	log.Printf("Found %d fighters to process", len(fighters))

	// Create a worker pool with a limited number of concurrent workers
	maxConcurrency := 8 // Adjust based on your system's capabilities
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	// Create database context with longer timeout for the entire operation
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 120*time.Minute)
	defer dbCancel()

	// Create a channel for results
	results := make(chan struct {
		fighter Fighter
		info    *scrapers.FighterExtraInfo
		err     error
	}, maxConcurrency)

	// Start result processor
	go func() {
		processedCount := 0
		startTime := time.Now()
		
		for result := range results {
			processedCount++
			
			// If there was an error, log it and continue
			if result.err != nil {
				log.Printf("%s: ❌ Failed - %v", result.fighter.Name, result.err)
				continue
			}

			if result.info == nil {
				log.Printf("%s: ❌ No data found", result.fighter.Name)
				continue
			}

			// Update fighter in database
			if err := updateFighterInDatabase(dbCtx, db, result.fighter, result.info); err != nil {
				log.Printf("Failed to update fighter %s: %v", result.fighter.Name, err)
			} else {
				log.Printf("%s: ✅ Updated successfully", result.fighter.Name)
			}
			
			// Log progress
			elapsedTime := time.Since(startTime)
			timePerFighter := elapsedTime / time.Duration(processedCount)
			log.Printf("Progress: %d/%d fighters processed (~%v per fighter)", 
				processedCount, len(fighters), timePerFighter.Round(time.Second))
		}
		
		log.Printf("🏁 Processing completed for %d fighters in %v", 
			processedCount, time.Since(startTime).Round(time.Second))
	}()

	// Create the fighter scraper
	fighterScraper := scrapers.NewWikiFighterScraper(scraperConfig)

	// Process fighters concurrently
	for _, fighter := range fighters {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore slot
		
		// Launch a goroutine for each fighter
		go func(f Fighter) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore slot when done
			
			// Create timeout context for this fighter
			fighterCtx, fighterCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer fighterCancel()
			
			// Run the scraping in another goroutine so we can handle timeout
			done := make(chan bool)
			var info *scrapers.FighterExtraInfo
			var scrapeErr error
			
			go func() {
				info, scrapeErr = fighterScraper.ScrapeExtraInfo(f.Name, f.WikiURL, "", f.Wins, f.Losses)
				select {
				case <-fighterCtx.Done():
					// Context was cancelled, do nothing
				case done <- true:
					// Succeeded in sending result
				}
			}()
			
			// Wait for either completion or timeout
			select {
			case <-done:
				// Processing completed normally
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: info, err: scrapeErr}
			case <-fighterCtx.Done():
				log.Printf("%s: ⚠️ Processing timed out after 30 seconds", f.Name)
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: nil, err: fmt.Errorf("timeout")}
			}
		}(fighter)
	}

	// Wait for all workers to complete
	wg.Wait()
	close(results) // Close results channel to signal completion
	
	log.Println("All workers have completed")
}

func updateFighterInDatabase(ctx context.Context, db *sql.DB, fighter Fighter, info *scrapers.FighterExtraInfo) error {
	// Update win methods if appropriate
	shouldUpdateWinMethods := false
	scrapedWinMethods := info.KOWins + info.SubWins + info.DecWins

	log.Printf("  - Win methods: KO:%d, Sub:%d, Dec:%d (total: %d)",
		info.KOWins, info.SubWins, info.DecWins, scrapedWinMethods)

	// Case: Fighter has wins, and scraped methods match the total
	if fighter.Wins > 0 && scrapedWinMethods == fighter.Wins {
		shouldUpdateWinMethods = true
	}

	// Update loss methods if appropriate
	shouldUpdateLossMethods := false
	scrapedLossMethods := info.KOLosses + info.SubLosses + info.DecLosses + info.DQLosses

	log.Printf("  - Loss methods: KO:%d, Sub:%d, Dec:%d, DQ:%d (total: %d)",
		info.KOLosses, info.SubLosses, info.DecLosses, info.DQLosses, scrapedLossMethods)

	// Case: Fighter has losses, and scraped methods match the total
	if fighter.Losses > 0 && scrapedLossMethods == fighter.Losses {
		shouldUpdateLossMethods = true
	}

	// Log No Contests information
	log.Printf("  - No Contests: %d", info.NoContests)

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback() // Will be ignored if transaction is committed

	// Update win methods if appropriate
	if shouldUpdateWinMethods {
		_, err = tx.ExecContext(ctx, `
			UPDATE fighters SET 
				ko_wins = $1, 
				sub_wins = $2, 
				dec_wins = $3, 
				updated_at = NOW()
			WHERE id = $4
		`, info.KOWins, info.SubWins, info.DecWins, fighter.ID)

		if err != nil {
			return fmt.Errorf("failed to update win methods: %v", err)
		}
		log.Printf("%s: Win methods updated", fighter.Name)
	} else {
		log.Printf("  - SKIPPING win method update")
	}

	// Update loss methods if appropriate
	if shouldUpdateLossMethods {
		_, err = tx.ExecContext(ctx, `
			UPDATE fighters SET 
				loss_by_ko = $1, 
				loss_by_sub = $2, 
				loss_by_dec = $3, 
				loss_by_dq = $4,
				updated_at = NOW()
			WHERE id = $5
		`, info.KOLosses, info.SubLosses, info.DecLosses, info.DQLosses, fighter.ID)

		if err != nil {
			return fmt.Errorf("failed to update loss methods: %v", err)
		}
		log.Printf("%s: Loss methods updated", fighter.Name)
	} else {
		log.Printf("  - SKIPPING loss method update")
	}

	// Update No Contests if available
	if info.NoContests > 0 {
		_, err = tx.ExecContext(ctx, `
			UPDATE fighters SET 
				no_contests = $1,
				updated_at = NOW()
			WHERE id = $2
		`, info.NoContests, fighter.ID)

		if err != nil {
			return fmt.Errorf("failed to update no contests: %v", err)
		}
		log.Printf("%s: No Contests updated", fighter.Name)
	}

	// Update wiki_url if we found it and it's currently NULL or empty
	if info.WikiURL != "" && (fighter.WikiURL == "" || fighter.WikiURL == "NULL") {
		_, err = tx.ExecContext(ctx, `
			UPDATE fighters SET 
				wiki_url = $1,
				updated_at = NOW()
			WHERE id = $2
		`, info.WikiURL, fighter.ID)

		if err != nil {
			return fmt.Errorf("failed to update wiki URL: %v", err)
		}
		log.Printf("%s: Wiki URL updated to %s", fighter.Name, info.WikiURL)
	}

	// Update fighting location if available
	if info.FightingOutOf != "" {
		_, err = tx.ExecContext(ctx, `
			UPDATE fighters SET 
				fighting_out_of = $1,
				updated_at = NOW()
			WHERE id = $2
		`, info.FightingOutOf, fighter.ID)

		if err != nil {
			return fmt.Errorf("failed to update fighter location info: %v", err)
		}
		log.Printf("%s: Fighting location updated", fighter.Name)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// getAllFighters retrieves all fighters from the database
func getAllFighters(db *sql.DB) ([]Fighter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query all fighters, prioritizing ones without win/loss methods
	rows, err := db.QueryContext(ctx, `
		SELECT 
			id, name, COALESCE(wiki_url, ''), wins, losses, 
			COALESCE(ko_wins, 0), COALESCE(sub_wins, 0), COALESCE(dec_wins, 0),
			COALESCE(loss_by_ko, 0), COALESCE(loss_by_sub, 0), COALESCE(loss_by_dec, 0), COALESCE(loss_by_dq, 0),
			COALESCE(no_contests, 0), COALESCE(fighting_out_of, '')
		FROM fighters
		ORDER BY 
			-- Prioritize fighters missing win/loss methods
			CASE WHEN (wins > 0 AND (ko_wins + sub_wins + dec_wins) = 0) THEN 0 ELSE 1 END,
			CASE WHEN (losses > 0 AND (loss_by_ko + loss_by_sub + loss_by_dec + loss_by_dq) = 0) THEN 0 ELSE 1 END,
			-- Then prioritize fighters missing location info
			CASE WHEN (fighting_out_of IS NULL OR fighting_out_of = '') THEN 0 ELSE 1 END,
			-- Add some randomness for variety
			RANDOM()
	`)
	if err != nil {
		return nil, fmt.Errorf("error querying fighters: %v", err)
	}
	defer rows.Close()

	var fighters []Fighter

	for rows.Next() {
		var f Fighter
		if err := rows.Scan(
			&f.ID, &f.Name, &f.WikiURL, &f.Wins, &f.Losses,
			&f.KOWins, &f.SubWins, &f.DecWins,
			&f.LossByKO, &f.LossBySub, &f.LossByDec, &f.LossByDQ,
			&f.NoContests, &f.FightingOutOf); err != nil {
			return nil, fmt.Errorf("error scanning fighter: %v", err)
		}
		fighters = append(fighters, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fighters: %v", err)
	}

	return fighters, nil
}