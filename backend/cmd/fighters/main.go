package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"
)

type ScraperApp struct {
	db            *sql.DB
	scraperConfig scrapers.ScraperConfig
	logger        *log.Logger
	timers        []*time.Timer
	timerMutex    sync.Mutex
}

type Event struct {
	ID   string
	Name string
	Date time.Time
}

type ActiveFighter struct {
	ID       string
	UFCID    string
	Name     string
	UFCURL   string
	Nickname string
}

func main() {
	cronFlag := flag.Bool("cron", false, "Run as a timer service with schedules based on event dates")
	fullFlag := flag.Bool("full", false, "Run full fighter scrape for all fighters")
	flag.Parse()

	logger := log.New(os.Stdout, "FIGHTER-SCRAPER: ", log.LstdFlags)
	logger.Println("🚀 Starting Fighter Scraper")

	if err := godotenv.Load(); err != nil {
		logger.Printf("Warning: .env file not found: %v", err)
	}

	if err := config.LoadConfig("config/config.json"); err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	db, err := connectToDatabase()
	if err != nil {
		logger.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()

	app := &ScraperApp{
		db:            db,
		scraperConfig: scrapers.DefaultConfig(),
		logger:        logger,
		timers:        make([]*time.Timer, 0),
	}

	if *cronFlag {
		logger.Println("Starting event-based timer service for active fighter updates")
		app.runEventBasedTimerService()
		return
	} else if *fullFlag {
		logger.Println("Running full fighter scrape for all fighters")
		app.runFullFighterScrape()
		return
	} else {
		logger.Println("Running incremental fighter refresh for active fighters")
		app.runActiveFighterScrape()
		return
	}
}

func connectToDatabase() (*sql.DB, error) {
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
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// getActiveFighters retrieves all fighters with status "Active" from the database
func (app *ScraperApp) getActiveFighters() ([]ActiveFighter, error) {
	query := `
		SELECT id, ufc_id, name, ufc_url, nickname
		FROM fighters
		WHERE status = 'Active' AND ufc_url IS NOT NULL AND ufc_url != ''
		ORDER BY name
	`
	
	rows, err := app.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active fighters: %w", err)
	}
	defer rows.Close()
	
	var activeFighters []ActiveFighter
	for rows.Next() {
		var fighter ActiveFighter
		var nickname sql.NullString
		
		if err := rows.Scan(&fighter.ID, &fighter.UFCID, &fighter.Name, &fighter.UFCURL, &nickname); err != nil {
			return nil, fmt.Errorf("failed to scan fighter row: %w", err)
		}
		
		if nickname.Valid {
			fighter.Nickname = nickname.String
		}
		
		activeFighters = append(activeFighters, fighter)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating fighters: %w", err)
	}
	
	app.logger.Printf("Found %d active fighters to refresh", len(activeFighters))
	return activeFighters, nil
}

// runActiveFighterScrape performs an incremental update of all active fighters
func (app *ScraperApp) runActiveFighterScrape() {
	startTime := time.Now()
	app.logger.Println("Starting incremental refresh of active fighters...")

	// Get all active fighters from the database
	activeFighters, err := app.getActiveFighters()
	if err != nil {
		app.logger.Fatalf("Error getting active fighters: %v", err)
	}

	if len(activeFighters) == 0 {
		app.logger.Println("No active fighters found in database")
		return
	}

	app.logger.Printf("Found %d active fighters to refresh", len(activeFighters))

	// Create scraper instance
	scraper := scrapers.NewFighterScraper(app.scraperConfig)

	// Convert to Fighter structs and scrape details
	fighters := make([]*scrapers.Fighter, len(activeFighters))
	for i, af := range activeFighters {
		fighters[i] = &scrapers.Fighter{
			UFCID:    af.UFCID,
			Name:     af.Name,
			UFCURL:   af.UFCURL,
			Nickname: af.Nickname,
			Status:   "Active", // We know they're active from the query
			Rankings: []scrapers.FighterRanking{}, // Initialize empty slice
		}
	}

	// Process fighters in parallel to get updated details
	scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer scrapeCancel()

	// Get detailed information for each fighter
	app.logger.Println("Retrieving updated information for active fighters...")
	enrichedFighters, err := app.processActiveFighters(scrapeCtx, fighters, scraper)
	if err != nil {
		app.logger.Fatalf("Error processing active fighters: %v", err)
	}

	// Update database with refreshed data
	app.processFighters(scrapeCtx, enrichedFighters)

	app.logger.Printf("🏁 Active fighter refresh completed in %v!", time.Since(startTime).Round(time.Second))
}

// processActiveFighters gets updated details for active fighters in parallel
func (app *ScraperApp) processActiveFighters(ctx context.Context, fighters []*scrapers.Fighter, scraper *scrapers.FighterScraper) ([]*scrapers.Fighter, error) {
	totalFighters := len(fighters)
	app.logger.Printf("Retrieving detailed information for %d active fighters...", totalFighters)

	// Create worker pool for parallel processing
	numWorkers := runtime.NumCPU()
	if numWorkers > 10 {
		numWorkers = 10 // Cap at a reasonable maximum
	}

	fighterCh := make(chan *scrapers.Fighter, len(fighters))
	var wg sync.WaitGroup

	// Stats counters
	var successCount, failureCount int32

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for fighter := range fighterCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Attempt to get fighter details with retries
				success := false
				attempts := 0
				maxAttempts := 3

				for !success && attempts < maxAttempts {
					err := scraper.GetFighterDetails(fighter)
					if err == nil {
						success = true
						atomic.AddInt32(&successCount, 1)
					} else {
						attempts++
						if attempts < maxAttempts {
							// Exponential backoff
							backoffTime := time.Duration(100*(1<<attempts)) * time.Millisecond
							if backoffTime > 1*time.Second {
								backoffTime = 1 * time.Second
							}
							time.Sleep(backoffTime)
						} else {
							atomic.AddInt32(&failureCount, 1)
							app.logger.Printf("Worker %d: Failed to get details for %s after %d attempts: %v",
								workerID, fighter.Name, attempts, err)
						}
					}
				}

				// Log progress periodically
				totalProcessed := atomic.LoadInt32(&successCount) + atomic.LoadInt32(&failureCount)
				if totalProcessed%20 == 0 || totalProcessed == int32(len(fighters)) {
					app.logger.Printf("Fighter details: %d/%d processed (%d successful, %d failed)",
						totalProcessed, len(fighters), successCount, failureCount)
				}
			}
		}(i)
	}

	// Send fighters to workers
	for _, fighter := range fighters {
		fighterCh <- fighter
	}
	close(fighterCh)

	// Wait for all workers to finish
	wg.Wait()

	app.logger.Printf("Completed getting fighter details: %d successful, %d failed",
		successCount, failureCount)

	// Update rankings for active fighters
	app.logger.Println("Updating rankings for active fighters...")
	rankingsByWeightClass, err := scraper.ScrapeRankings()
	if err != nil {
		app.logger.Printf("Warning: Error scraping rankings: %v", err)
	} else {
		// Apply rankings to fighters
		app.updateFighterRankings(fighters, rankingsByWeightClass)
	}

	return fighters, nil
}

// updateFighterRankings applies current rankings to the fighters
func (app *ScraperApp) updateFighterRankings(fighters []*scrapers.Fighter, rankingsByWeightClass map[string]map[string]string) {
	for _, fighter := range fighters {
		// Skip if fighter is no longer active (might have been updated during scraping)
		if fighter.Status == "Retired" || fighter.Status == "Not Fighting" {
			fighter.Ranking = "Unranked"
			fighter.Rankings = []scrapers.FighterRanking{}
			continue
		}

		// Reset rankings
		fighter.Rankings = []scrapers.FighterRanking{}
		fighter.Ranking = "Unranked"

		// For each weight class in the rankings
		for weightClass, rankingsByFighterID := range rankingsByWeightClass {
			// If this fighter has a ranking in this weight class
			if ranking, exists := rankingsByFighterID[fighter.UFCID]; exists {
				// Add this ranking to the fighter's rankings
				newRanking := scrapers.FighterRanking{
					WeightClass: weightClass,
					Rank:        ranking,
				}
				fighter.Rankings = append(fighter.Rankings, newRanking)

				// Update the primary ranking field if:
				// 1. This is the fighter's primary weight class, or
				// 2. They don't have a ranking yet, or
				// 3. This is a championship ranking
				if fighter.WeightClass == weightClass ||
					fighter.Ranking == "Unranked" ||
					ranking == "Champion" || ranking == "Interim Champion" {
					fighter.Ranking = ranking
				}
			}
		}
	}
}

func (app *ScraperApp) runEventBasedTimerService() {
	app.logger.Println("Starting event-based timer service for active fighter updates")

	app.scheduleJobsForEvents()

	app.logger.Println("Timer scheduler started")
	app.logger.Println("Active fighter updates will run exactly 24 hours after each event")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	app.logger.Println("Shutting down fighter scraper service...")
	
	app.timerMutex.Lock()
	for _, timer := range app.timers {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
	app.timerMutex.Unlock()
	
	if err := app.db.Close(); err != nil {
		app.logger.Printf("Error closing database connection: %v", err)
	}
	
	app.logger.Println("Fighter scraper service stopped gracefully")
}

func (app *ScraperApp) scheduleJobsForEvents() {
	// Get all upcoming events (status = 'Upcoming')
	upcomingEvents, err := app.getUpcomingEvents()
	if err != nil {
		app.logger.Printf("Error getting upcoming events: %v", err)
		return
	}

	scheduledCount := 0
	
	now := time.Now().UTC()
	app.logger.Printf("Current time: %s UTC", now.Format(time.RFC3339))

	for _, event := range upcomingEvents {
		// Schedule fighter update exactly 24 hours after the event
		updateTime := event.Date.Add(24 * time.Hour)
		
		// Skip events that would be scheduled in the past
		if updateTime.Before(now) {
			app.logger.Printf("Skipping past event: %s (event date: %s UTC, update time: %s UTC)", 
				event.Name, event.Date.Format(time.RFC3339), updateTime.Format(time.RFC3339))
			continue
		}

		duration := updateTime.Sub(now)
		
		// Capture values for the closure
		eventName := event.Name
		eventDate := event.Date
		
		timer := time.AfterFunc(duration, func() {
			app.logger.Printf("Running active fighter update for event: %s (event date: %s UTC)", 
				eventName, eventDate.Format(time.RFC3339))
			// Run the active fighter scrape
			app.runActiveFighterScrape()
		})
		
		app.timerMutex.Lock()
		app.timers = append(app.timers, timer)
		app.timerMutex.Unlock()

		scheduledCount++
		app.logger.Printf("Scheduled active fighter update for %s in %s at %s UTC (24h after event: %s UTC)", 
			event.Name, 
			duration.Round(time.Second).String(), 
			updateTime.Format(time.RFC3339), 
			event.Date.Format(time.RFC3339))
	}

	app.logger.Printf("Successfully scheduled %d active fighter updates for upcoming events", scheduledCount)
}

// getUpcomingEvents retrieves all events with status 'Upcoming' from the database
func (app *ScraperApp) getUpcomingEvents() ([]Event, error) {
	query := `
		SELECT id, name, event_date
		FROM events
		WHERE status = 'Upcoming'
		ORDER BY event_date ASC
	`
	
	rows, err := app.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query upcoming events: %w", err)
	}
	defer rows.Close()
	
	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Name, &event.Date); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}
		
		// Ensure the date is in UTC
		event.Date = event.Date.UTC()
		events = append(events, event)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}
	
	app.logger.Printf("Found %d upcoming events", len(events))
	return events, nil
}

func (app *ScraperApp) processFighters(ctx context.Context, fighters []*scrapers.Fighter) {
    if ctx.Err() != nil {
        app.logger.Printf("Context canceled before processing fighters: %v", ctx.Err())
        return
    }
	var insertCount, updateCount, errorCount, rankingsInserted int64
	
	numWorkers := runtime.NumCPU() * 2
	if numWorkers > 5 {
		numWorkers = 5
	}
	
	batchSize := 20
	numBatches := (len(fighters) + batchSize - 1) / batchSize
	fighterBatchCh := make(chan []*scrapers.Fighter, numBatches)
	
	var wg sync.WaitGroup
	
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer dbCancel()
			
			for fighterBatch := range fighterBatchCh {
				var tx *sql.Tx
				var committed bool
				
				var beginErr error
				for retries := 0; retries < 3; retries++ {
					tx, beginErr = app.db.BeginTx(dbCtx, nil)
					if beginErr == nil {
						break
					}
					
					app.logger.Printf("Worker %d: Retry %d - Failed to begin transaction: %v", 
						workerID, retries+1, beginErr)
					time.Sleep(time.Duration(2<<retries) * time.Second)
				}
				
				if beginErr != nil {
					app.logger.Printf("Worker %d: Failed to begin transaction after retries: %v", 
						workerID, beginErr)
					atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
					continue
				}
				
				defer func() {
					if tx != nil && !committed {
						_ = tx.Rollback()
						tx = nil
					}
				}()
				
				insertStmt, err := tx.PrepareContext(dbCtx, `
					INSERT INTO fighters (
						ufc_id, name, nickname, weight_class, status, rank, wins, losses, draws, ufc_url,
						age, height, weight, reach, ko_wins, sub_wins, dec_wins, nationality,
						created_at, updated_at
					) VALUES (
						$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $19
					) RETURNING id`)
				if err != nil {
					app.logger.Printf("Worker %d: Failed to prepare insert statement: %v", workerID, err)
					_ = tx.Rollback()
					tx = nil
					atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
					continue
				}
				defer insertStmt.Close()
				
				updateStmt, err := tx.PrepareContext(dbCtx, `
					UPDATE fighters SET 
						name = $1,
						nickname = $2,
						weight_class = $3,
						status = $4,
						rank = $5,
						wins = $6,
						losses = $7,
						draws = $8,
						ufc_url = $9,
						age = $10,
						height = $11,
						weight = $12,
						reach = $13,
						ko_wins = $14,
						sub_wins = $15,
						dec_wins = $16,
						nationality = $17,
						updated_at = $18
					WHERE id = $19`)
				if err != nil {
					app.logger.Printf("Worker %d: Failed to prepare update statement: %v", workerID, err)
					_ = tx.Rollback()
					tx = nil
					atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
					continue
				}
				defer updateStmt.Close()
				
				batchInsertCount := 0
				batchUpdateCount := 0
				batchRankingsCount := 0
				batchSuccess := true
				now := time.Now()
				
				for _, fighter := range fighterBatch {
					wins := fighter.KOWins + fighter.SubWins + fighter.DecWins
					losses := 0
					
					if fighter.Record != "" {
						parts := strings.Split(fighter.Record, "-")
						if len(parts) >= 2 {
							losses, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
						}
					}
					
					var existingID string
					
					err = tx.QueryRowContext(dbCtx,
						"SELECT id FROM fighters WHERE ufc_id = $1", fighter.UFCID).Scan(&existingID)
					
					if err != nil && err != sql.ErrNoRows {
						app.logger.Printf("Worker %d: Error checking for existing fighter %s: %v", 
							workerID, fighter.Name, err)
						batchSuccess = false
						break
					}
					
					if err == nil {
						_, err = updateStmt.ExecContext(dbCtx,
							fighter.Name,
							fighter.Nickname,
							fighter.WeightClass,
							fighter.Status,
							fighter.Ranking,
							wins,
							losses,
							fighter.Draws,
							fighter.UFCURL,
							fighter.Age,
							fighter.Height,
							fighter.Weight,
							fighter.Reach,
							fighter.KOWins,
							fighter.SubWins,
							fighter.DecWins,
							fighter.Nationality,
							now,
							existingID,
						)
						
						if err != nil {
							app.logger.Printf("Worker %d: Failed to update fighter %s: %v", 
								workerID, fighter.Name, err)
							batchSuccess = false
							break
						}
						
						batchUpdateCount++
					} else {
						err = insertStmt.QueryRowContext(dbCtx,
							fighter.UFCID,
							fighter.Name,
							fighter.Nickname,
							fighter.WeightClass,
							fighter.Status,
							fighter.Ranking,
							wins,
							losses,
							fighter.Draws,
							fighter.UFCURL,
							fighter.Age,
							fighter.Height,
							fighter.Weight,
							fighter.Reach,
							fighter.KOWins,
							fighter.SubWins,
							fighter.DecWins,
							fighter.Nationality,
							now,
						).Scan(&existingID)
						
						if err != nil {
							app.logger.Printf("Worker %d: Failed to insert fighter %s: %v", 
								workerID, fighter.Name, err)
							batchSuccess = false
							break
						}
						
						batchInsertCount++
					}
					
					if len(fighter.Rankings) > 0 {
						_, err = tx.ExecContext(dbCtx,
							"DELETE FROM fighter_rankings WHERE fighter_id = $1",
							existingID)
						
						if err != nil {
							app.logger.Printf("Worker %d: Failed to delete existing rankings for fighter %s: %v", 
								workerID, fighter.Name, err)
							batchSuccess = false
							break
						}
						
						rankingValues := make([]string, 0, len(fighter.Rankings))
						rankingArgs := make([]interface{}, 0, len(fighter.Rankings)*4)
						argCount := 1
						
						for _, ranking := range fighter.Rankings {
							placeholder := fmt.Sprintf("($%d, $%d, $%d, $%d)",
								argCount, argCount+1, argCount+2, argCount+3)
							rankingValues = append(rankingValues, placeholder)
							
							rankingArgs = append(rankingArgs, 
								existingID, 
								ranking.WeightClass, 
								ranking.Rank, 
								now)
							
							argCount += 4
							batchRankingsCount++
						}
						
						if len(rankingValues) > 0 {
							rankingSQL := fmt.Sprintf(`
								INSERT INTO fighter_rankings (
									fighter_id, weight_class, rank, created_at
								) VALUES %s`, strings.Join(rankingValues, ", "))
							
							_, err = tx.ExecContext(dbCtx, rankingSQL, rankingArgs...)
							
							if err != nil {
								app.logger.Printf("Worker %d: Failed to insert rankings for fighter %s: %v", 
									workerID, fighter.Name, err)
								batchSuccess = false
								break
							}
						}
					}
				}
				
				if batchSuccess {
					if err = tx.Commit(); err != nil {
						app.logger.Printf("Worker %d: Failed to commit transaction: %v", workerID, err)
						_ = tx.Rollback()
						tx = nil
						atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
						continue
					}
					
					committed = true
					tx = nil
					
					atomic.AddInt64(&insertCount, int64(batchInsertCount))
					atomic.AddInt64(&updateCount, int64(batchUpdateCount))
					atomic.AddInt64(&rankingsInserted, int64(batchRankingsCount))
					
					totalProcessed := atomic.LoadInt64(&insertCount) + atomic.LoadInt64(&updateCount)
					if totalProcessed%50 == 0 {
						app.logger.Printf("Progress: %d/%d fighters processed", totalProcessed, len(fighters))
					}
				} else {
					_ = tx.Rollback()
					tx = nil
					atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
					continue
				}
			}
			
			app.logger.Printf("Worker %d completed", workerID)
		}(i)
	}
	
	for i := 0; i < len(fighters); i += batchSize {
		end := i + batchSize
		if end > len(fighters) {
			end = len(fighters)
		}
		
		fighterBatch := fighters[i:end]
		fighterBatchCh <- fighterBatch
	}
	
	close(fighterBatchCh)
	
	wg.Wait()
	
	app.logger.Printf("Scraping completed! Saved %d new fighters, updated %d existing fighters with %d total rankings. Errors: %d",
		insertCount, updateCount, rankingsInserted, errorCount)
}

func (app *ScraperApp) runFullFighterScrape() {
	startTime := time.Now()
	app.logger.Println("Starting full fighter scrape...")

	scraper := scrapers.NewFighterScraper(app.scraperConfig)

	scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer scrapeCancel()

	fighters, err := scraper.ScrapeAllFighters(scrapeCtx)
	if err != nil {
		app.logger.Fatalf("Error scraping fighters: %v", err)
	}

	app.logger.Printf("Retrieved %d fighters total, now processing...", len(fighters))
	
	app.processFighters(scrapeCtx, fighters)

	app.logger.Printf("🏁 Full fighter scrape completed in %v!", time.Since(startTime).Round(time.Second))
}