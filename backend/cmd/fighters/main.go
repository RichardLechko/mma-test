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
	"github.com/robfig/cron/v3"
	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"
)

type ScraperApp struct {
	db            *sql.DB
	scraperConfig scrapers.ScraperConfig  // Changed from pointer to value
	logger        *log.Logger
}

func main() {
	fullScrapeFlag := flag.Bool("full", false, "Run a full fighter scrape")
	cronFlag := flag.Bool("cron", false, "Run as a cron job service")
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
		scraperConfig: scrapers.DefaultConfig(),  // Now matches the type
		logger:        logger,
	}

	if *fullScrapeFlag {
		logger.Println("Running full fighter scrape")
		app.runFighterScrape()
		return
	} else if *cronFlag {
		logger.Println("Starting cron service mode")
		app.runCronService()
		return
	} else {
		logger.Println("Running standard fighter scrape")
		app.runFighterScrape()
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

	// Optimize connection pool settings
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

func (app *ScraperApp) runFighterScrape() {
	startTime := time.Now()
	app.logger.Println("Starting fighter scrape...")

	scraper := scrapers.NewFighterScraper(app.scraperConfig)  // Now matches the expected type

	// Use a longer context for the scraping operation
	scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer scrapeCancel()

	fighters, err := scraper.ScrapeAllFighters(scrapeCtx)
	if err != nil {
		app.logger.Fatalf("Error scraping fighters: %v", err)
	}

	app.logger.Printf("Retrieved %d fighters total, now processing...", len(fighters))
	
	// Process and save the fighters
	app.processFighters(scrapeCtx, fighters)

	app.logger.Printf("🏁 Fighter scrape completed in %v!", time.Since(startTime).Round(time.Second))
}

func (app *ScraperApp) runCronService() {
	app.logger.Println("Starting cron service for fighter updates")

	c := cron.New(
		cron.WithLogger(cron.VerbosePrintfLogger(app.logger)),
		cron.WithLocation(time.UTC),
	)

	// Run fighter update weekly on Sunday at 03:00 UTC
	if _, err := c.AddFunc("0 3 * * 0", app.runFighterScrape); err != nil {
		app.logger.Fatalf("Failed to add weekly fighter update job: %v", err)
	}

	c.Start()
	app.logger.Println("Cron scheduler started with the following schedule:")
	app.logger.Println("- Weekly fighter updates every Sunday at 03:00 UTC")

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	app.logger.Println("Shutting down scraper service...")
	
	c.Stop()
	
	if err := app.db.Close(); err != nil {
		app.logger.Printf("Error closing database connection: %v", err)
	}
	
	app.logger.Println("Fighter scraper service stopped gracefully")
}

func (app *ScraperApp) processFighters(ctx context.Context, fighters []*scrapers.Fighter) {
    // Check for context cancellation at the start
    if ctx.Err() != nil {
        app.logger.Printf("Context canceled before processing fighters: %v", ctx.Err())
        return
    }
	// Track statistics with atomic counters
	var insertCount, updateCount, errorCount, rankingsInserted int64
	
	// Process fighters in parallel using a worker pool with batching
	numWorkers := runtime.NumCPU() * 2
	if numWorkers > 5 {
		numWorkers = 5
	}
	
	// Create a channel to distribute work - use batches for better efficiency
	batchSize := 20
	numBatches := (len(fighters) + batchSize - 1) / batchSize
	fighterBatchCh := make(chan []*scrapers.Fighter, numBatches)
	
	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup
	
	// Launch worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Each worker gets its own database connection context
			dbCtx, dbCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer dbCancel()
			
			// Process fighter batches from the channel
			for fighterBatch := range fighterBatchCh {
				var tx *sql.Tx
				var committed bool
				
				// Use a retry mechanism for transaction begin
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
				
				// Ensure the transaction is either committed or rolled back
				defer func() {
					if tx != nil && !committed {
						_ = tx.Rollback()
						tx = nil
					}
				}()
				
				// Prepare statements for better performance
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
				
				// Process each fighter in the batch
				batchInsertCount := 0
				batchUpdateCount := 0
				batchRankingsCount := 0
				batchSuccess := true
				now := time.Now()
				
				for _, fighter := range fighterBatch {
					// Use fighter.Draws directly instead of parsing it
					wins := fighter.KOWins + fighter.SubWins + fighter.DecWins
					losses := 0
					
					// Only parse losses from the record string
					if fighter.Record != "" {
						parts := strings.Split(fighter.Record, "-")
						if len(parts) >= 2 {
							losses, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
						}
					}
					
					// Check if fighter exists
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
						// Update existing fighter
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
						// Insert new fighter
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
					
					// Process rankings if they exist
					if len(fighter.Rankings) > 0 {
						// First delete any existing rankings for this fighter
						_, err = tx.ExecContext(dbCtx,
							"DELETE FROM fighter_rankings WHERE fighter_id = $1",
							existingID)
						
						if err != nil {
							app.logger.Printf("Worker %d: Failed to delete existing rankings for fighter %s: %v", 
								workerID, fighter.Name, err)
							batchSuccess = false
							break
						}
						
						// Use a single bulk insert for all rankings
						// Build values string for a bulk insert
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
					
					// Only set committed = true if the commit succeeded
					committed = true
					tx = nil
					
					// Update global counters
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
	
	// Group fighters into batches and send to the workers
	for i := 0; i < len(fighters); i += batchSize {
		end := i + batchSize
		if end > len(fighters) {
			end = len(fighters)
		}
		
		fighterBatch := fighters[i:end]
		fighterBatchCh <- fighterBatch
	}
	
	close(fighterBatchCh)
	
	// Wait for all workers to finish
	wg.Wait()
	
	app.logger.Printf("Scraping completed! Saved %d new fighters, updated %d existing fighters with %d total rankings. Errors: %d",
		insertCount, updateCount, rankingsInserted, errorCount)
}