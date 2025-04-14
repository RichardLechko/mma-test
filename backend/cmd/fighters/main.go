package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

	// Optimize connection pool settings
	db.SetMaxOpenConns(50)  // Increased from 25
	db.SetMaxIdleConns(20)  // Increased from 10
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute) // Add idle timeout

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Connected to database")

	config := scrapers.DefaultConfig()
	scraper := scrapers.NewFighterScraper(config)

	log.Println("Scraping fighters from UFC website...")

	// Use a longer context for the scraping operation
	scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer scrapeCancel()

	fighters, err := scraper.ScrapeAllFighters(scrapeCtx)
	if err != nil {
		log.Fatalf("Error scraping fighters: %v", err)
	}

	log.Printf("Retrieved %d fighters total, now processing...", len(fighters))
	
	// Track statistics with atomic counters
	var insertCount, updateCount, errorCount, rankingsInserted int64
	
	// Process fighters in parallel using a worker pool with batching
	numWorkers := runtime.NumCPU() * 2
	if numWorkers > 16 {
		numWorkers = 16 // Cap at a reasonable maximum
	}
	
	// Create a channel to distribute work - use batches for better efficiency
	batchSize := 10 // Process multiple fighters per transaction
	numBatches := (len(fighters) + batchSize - 1) / batchSize
	fighterBatchCh := make(chan []*scrapers.Fighter, numBatches)
	
	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup
	
	// Launch worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Each worker gets its own database transaction context
			dbCtx, dbCancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer dbCancel()
			
			// Process fighter batches from the channel
			for fighterBatch := range fighterBatchCh {
				// Start a new transaction for each batch
				tx, err := db.BeginTx(dbCtx, nil)
				if err != nil {
					log.Printf("Worker %d: Failed to begin transaction: %v", workerID, err)
					atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
					continue
				}
				
				// Ensure the transaction is either committed or rolled back
				var committed bool
				defer func() {
					if tx != nil && !committed {
						_ = tx.Rollback()
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
					log.Printf("Worker %d: Failed to prepare insert statement: %v", workerID, err)
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
					log.Printf("Worker %d: Failed to prepare update statement: %v", workerID, err)
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
					// Parse record to get wins, losses, draws
					wins, losses, draws := 0, 0, 0
					
					if fighter.Record != "" {
						parts := strings.Split(fighter.Record, "-")
						if len(parts) >= 3 {
							wins, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
							losses, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
							draws, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
						}
					}
					
					// Check if fighter exists
					var existingID string
					
					err = tx.QueryRowContext(dbCtx,
						"SELECT id FROM fighters WHERE ufc_id = $1", fighter.UFCID).Scan(&existingID)
					
					if err != nil && err != sql.ErrNoRows {
						log.Printf("Worker %d: Error checking for existing fighter %s: %v", 
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
							draws,
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
							log.Printf("Worker %d: Failed to update fighter %s: %v", 
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
							draws,
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
							log.Printf("Worker %d: Failed to insert fighter %s: %v", 
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
							log.Printf("Worker %d: Failed to delete existing rankings for fighter %s: %v", 
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
								log.Printf("Worker %d: Failed to insert rankings for fighter %s: %v", 
									workerID, fighter.Name, err)
								batchSuccess = false
								break
							}
						}
					}
				}
				
				// Commit the transaction if all operations were successful
				if batchSuccess {
					if err = tx.Commit(); err != nil {
						log.Printf("Worker %d: Failed to commit transaction: %v", workerID, err)
						_ = tx.Rollback()
						tx = nil
						atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
						continue
					}
					
					// Update global counters
					atomic.AddInt64(&insertCount, int64(batchInsertCount))
					atomic.AddInt64(&updateCount, int64(batchUpdateCount))
					atomic.AddInt64(&rankingsInserted, int64(batchRankingsCount))
					
					totalProcessed := atomic.LoadInt64(&insertCount) + atomic.LoadInt64(&updateCount)
					if totalProcessed%50 == 0 {
						log.Printf("Progress: %d/%d fighters processed", totalProcessed, len(fighters))
					}
				} else {
					// Roll back the transaction if any operation failed
					_ = tx.Rollback()
					tx = nil
					atomic.AddInt64(&errorCount, int64(len(fighterBatch)))
					continue
				}
				
				committed = true
				tx = nil
			}
			
			log.Printf("Worker %d completed", workerID)
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
	
	log.Printf("Scraping completed! Saved %d new fighters, updated %d existing fighters with %d total rankings. Errors: %d",
		insertCount, updateCount, rankingsInserted, errorCount)
}