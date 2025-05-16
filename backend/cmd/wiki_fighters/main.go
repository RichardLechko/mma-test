package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"
)

type ScraperApp struct {
	db            *sql.DB
	scraperConfig *scrapers.ScraperConfig
	logger        *log.Logger
	timers        []*time.Timer
	timerMutex    sync.Mutex
}

type Event struct {
	ID   string
	Name string
	Date time.Time
}

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

type ActiveFighter struct {
	ID       string
	Name     string
	WikiURL  string
	Wins     int
	Losses   int
	KOWins   int
	SubWins  int
	DecWins  int
	LossByKO int
	LossBySub int
	LossByDec int
	LossByDQ int
	NoContests int
	FightingOutOf string
}

func main() {
	cronFlag := flag.Bool("cron", false, "Run as a timer service with schedules based on event dates")
	fullFlag := flag.Bool("full", false, "Run full wiki fighter scrape for all fighters")
	flag.Parse()

	logger := log.New(os.Stdout, "WIKI-FIGHTER-SCRAPER: ", log.LstdFlags)
	logger.Println("🚀 Starting Wiki Fighter Scraper")

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
		db: db,
		scraperConfig: &scrapers.ScraperConfig{
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		logger: logger,
		timers: make([]*time.Timer, 0),
	}

	if *cronFlag {
		logger.Println("Starting event-based timer service for active fighter wiki updates")
		app.runEventBasedTimerService()
		return
	} else if *fullFlag {
		logger.Println("Running full wiki fighter scrape for all fighters")
		app.runFullWikiFighterScrape()
		return
	} else {
		logger.Println("Running incremental wiki fighter scrape for active fighters")
		app.runActiveWikiFighterScrape()
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

func (app *ScraperApp) runWikiFighterScrape() {
	startTime := time.Now()
	app.logger.Println("Starting wiki fighter data scrape...")

	fighters, err := getAllFighters(app.db)
	if err != nil {
		app.logger.Fatalf("Error getting fighters: %v", err)
	}

	app.logger.Printf("Found %d fighters to process", len(fighters))

	maxConcurrency := 8
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 120*time.Minute)
	defer dbCancel()

	results := make(chan struct {
		fighter Fighter
		info    *scrapers.FighterExtraInfo
		err     error
	}, maxConcurrency)

	go func() {
		processedCount := 0
		startTime := time.Now()
		
		for result := range results {
			processedCount++
			
			if result.err != nil {
				app.logger.Printf("%s: ❌ Failed - %v", result.fighter.Name, result.err)
				continue
			}

			if result.info == nil {
				app.logger.Printf("%s: ❌ No data found", result.fighter.Name)
				continue
			}

			if err := updateFighterInDatabase(dbCtx, app.db, result.fighter, result.info); err != nil {
				app.logger.Printf("Failed to update fighter %s: %v", result.fighter.Name, err)
			} else {
				app.logger.Printf("%s: ✅ Updated successfully", result.fighter.Name)
			}
			
			elapsedTime := time.Since(startTime)
			timePerFighter := elapsedTime / time.Duration(processedCount)
			app.logger.Printf("Progress: %d/%d fighters processed (~%v per fighter)", 
				processedCount, len(fighters), timePerFighter.Round(time.Second))
		}
		
		app.logger.Printf("🏁 Processing completed for %d fighters in %v", 
			processedCount, time.Since(startTime).Round(time.Second))
	}()

	fighterScraper := scrapers.NewWikiFighterScraper(app.scraperConfig)

	for _, fighter := range fighters {
		wg.Add(1)
		sem <- struct{}{}
		
		go func(f Fighter) {
			defer wg.Done()
			defer func() { <-sem }()
			
			fighterCtx, fighterCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer fighterCancel()
			
			done := make(chan bool)
			var info *scrapers.FighterExtraInfo
			var scrapeErr error
			
			go func() {
				info, scrapeErr = fighterScraper.ScrapeExtraInfo(f.Name, f.WikiURL, "", f.Wins, f.Losses)
				select {
				case <-fighterCtx.Done():
				case done <- true:
				}
			}()
			
			select {
			case <-done:
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: info, err: scrapeErr}
			case <-fighterCtx.Done():
				app.logger.Printf("%s: ⚠️ Processing timed out after 30 seconds", f.Name)
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: nil, err: fmt.Errorf("timeout")}
			}
		}(fighter)
	}

	wg.Wait()
	close(results)
	
	app.logger.Printf("🏁 Wiki fighter scrape completed in %v!", time.Since(startTime).Round(time.Second))
}

func (app *ScraperApp) runEventBasedTimerService() {
	app.logger.Println("Starting event-based timer service for active fighter wiki updates")

	app.scheduleJobsForEvents()

	app.logger.Println("Timer scheduler started")
	app.logger.Println("Active fighter wiki updates will run exactly 24 hours after each event")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	app.logger.Println("Shutting down wiki fighter scraper service...")
	
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
	
	app.logger.Println("Wiki fighter scraper service stopped gracefully")
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
		// Schedule wiki fighter update exactly 24 hours after the event
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
			app.logger.Printf("Running active fighter wiki update for event: %s (event date: %s UTC)", 
				eventName, eventDate.Format(time.RFC3339))
			// Run the active fighter wiki scrape
			app.runActiveWikiFighterScrape()
		})
		
		app.timerMutex.Lock()
		app.timers = append(app.timers, timer)
		app.timerMutex.Unlock()

		scheduledCount++
		app.logger.Printf("Scheduled active fighter wiki update for %s in %s at %s UTC (24h after event: %s UTC)", 
			event.Name, 
			duration.Round(time.Second).String(), 
			updateTime.Format(time.RFC3339), 
			event.Date.Format(time.RFC3339))
	}

	app.logger.Printf("Successfully scheduled %d active fighter wiki updates for upcoming events", scheduledCount)
}

func (app *ScraperApp) getUpcomingAndRecentEvents() ([]Event, error) {
	oneMonthAgo := time.Now().UTC().AddDate(0, -1, 0)
	oneYearFromNow := time.Now().UTC().AddDate(1, 0, 0)
	
	query := `
		SELECT id, name, event_date
		FROM events
		WHERE event_date BETWEEN $1 AND $2
		ORDER BY event_date ASC
	`
	
	rows, err := app.db.Query(query, oneMonthAgo, oneYearFromNow)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()
	
	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Name, &event.Date); err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}
		
		event.Date = event.Date.UTC()
		events = append(events, event)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}
	
	app.logger.Printf("Found %d upcoming and recent events", len(events))
	return events, nil
}

func updateFighterInDatabase(ctx context.Context, db *sql.DB, fighter Fighter, info *scrapers.FighterExtraInfo) error {
	shouldUpdateWinMethods := false
	scrapedWinMethods := info.KOWins + info.SubWins + info.DecWins

	log.Printf("  - Win methods: KO:%d, Sub:%d, Dec:%d (total: %d)",
		info.KOWins, info.SubWins, info.DecWins, scrapedWinMethods)

	if fighter.Wins > 0 && scrapedWinMethods == fighter.Wins {
		shouldUpdateWinMethods = true
	}

	shouldUpdateLossMethods := false
	scrapedLossMethods := info.KOLosses + info.SubLosses + info.DecLosses + info.DQLosses

	log.Printf("  - Loss methods: KO:%d, Sub:%d, Dec:%d, DQ:%d (total: %d)",
		info.KOLosses, info.SubLosses, info.DecLosses, info.DQLosses, scrapedLossMethods)

	if fighter.Losses > 0 && scrapedLossMethods == fighter.Losses {
		shouldUpdateLossMethods = true
	}

	log.Printf("  - No Contests: %d", info.NoContests)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func getAllFighters(db *sql.DB) ([]Fighter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT 
			id, name, COALESCE(wiki_url, ''), wins, losses, 
			COALESCE(ko_wins, 0), COALESCE(sub_wins, 0), COALESCE(dec_wins, 0),
			COALESCE(loss_by_ko, 0), COALESCE(loss_by_sub, 0), COALESCE(loss_by_dec, 0), COALESCE(loss_by_dq, 0),
			COALESCE(no_contests, 0), COALESCE(fighting_out_of, '')
		FROM fighters
		ORDER BY 
			CASE WHEN (wins > 0 AND (ko_wins + sub_wins + dec_wins) = 0) THEN 0 ELSE 1 END,
			CASE WHEN (losses > 0 AND (loss_by_ko + loss_by_sub + loss_by_dec + loss_by_dq) = 0) THEN 0 ELSE 1 END,
			CASE WHEN (fighting_out_of IS NULL OR fighting_out_of = '') THEN 0 ELSE 1 END,
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

func (app *ScraperApp) getActiveFightersForWiki() ([]Fighter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query only active fighters
	rows, err := app.db.QueryContext(ctx, `
		SELECT 
			id, name, COALESCE(wiki_url, ''), wins, losses, 
			COALESCE(ko_wins, 0), COALESCE(sub_wins, 0), COALESCE(dec_wins, 0),
			COALESCE(loss_by_ko, 0), COALESCE(loss_by_sub, 0), COALESCE(loss_by_dec, 0), COALESCE(loss_by_dq, 0),
			COALESCE(no_contests, 0), COALESCE(fighting_out_of, '')
		FROM fighters
		WHERE status = 'Active'
		ORDER BY 
			CASE WHEN (wins > 0 AND (ko_wins + sub_wins + dec_wins) = 0) THEN 0 ELSE 1 END,
			CASE WHEN (losses > 0 AND (loss_by_ko + loss_by_sub + loss_by_dec + loss_by_dq) = 0) THEN 0 ELSE 1 END,
			CASE WHEN (fighting_out_of IS NULL OR fighting_out_of = '') THEN 0 ELSE 1 END,
			RANDOM()
	`)
	if err != nil {
		return nil, fmt.Errorf("error querying active fighters: %v", err)
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

	app.logger.Printf("Found %d active fighters to process", len(fighters))
	return fighters, nil
}

func (app *ScraperApp) runActiveWikiFighterScrape() {
	startTime := time.Now()
	app.logger.Println("Starting incremental wiki fighter data scrape for active fighters...")

	fighters, err := app.getActiveFightersForWiki()
	if err != nil {
		app.logger.Fatalf("Error getting active fighters: %v", err)
	}

	if len(fighters) == 0 {
		app.logger.Println("No active fighters found in database")
		return
	}

	app.logger.Printf("Found %d active fighters to process", len(fighters))

	maxConcurrency := 8
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 120*time.Minute)
	defer dbCancel()

	results := make(chan struct {
		fighter Fighter
		info    *scrapers.FighterExtraInfo
		err     error
	}, maxConcurrency)

	go func() {
		processedCount := 0
		startTime := time.Now()
		
		for result := range results {
			processedCount++
			
			if result.err != nil {
				app.logger.Printf("%s: ❌ Failed - %v", result.fighter.Name, result.err)
				continue
			}

			if result.info == nil {
				app.logger.Printf("%s: ❌ No data found", result.fighter.Name)
				continue
			}

			if err := updateFighterInDatabase(dbCtx, app.db, result.fighter, result.info); err != nil {
				app.logger.Printf("Failed to update fighter %s: %v", result.fighter.Name, err)
			} else {
				app.logger.Printf("%s: ✅ Updated successfully", result.fighter.Name)
			}
			
			elapsedTime := time.Since(startTime)
			timePerFighter := elapsedTime / time.Duration(processedCount)
			app.logger.Printf("Progress: %d/%d active fighters processed (~%v per fighter)", 
				processedCount, len(fighters), timePerFighter.Round(time.Second))
		}
		
		app.logger.Printf("🏁 Active fighter processing completed for %d fighters in %v", 
			processedCount, time.Since(startTime).Round(time.Second))
	}()

	fighterScraper := scrapers.NewWikiFighterScraper(app.scraperConfig)

	for _, fighter := range fighters {
		wg.Add(1)
		sem <- struct{}{}
		
		go func(f Fighter) {
			defer wg.Done()
			defer func() { <-sem }()
			
			fighterCtx, fighterCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer fighterCancel()
			
			done := make(chan bool)
			var info *scrapers.FighterExtraInfo
			var scrapeErr error
			
			go func() {
				info, scrapeErr = fighterScraper.ScrapeExtraInfo(f.Name, f.WikiURL, "", f.Wins, f.Losses)
				select {
				case <-fighterCtx.Done():
				case done <- true:
				}
			}()
			
			select {
			case <-done:
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: info, err: scrapeErr}
			case <-fighterCtx.Done():
				app.logger.Printf("%s: ⚠️ Processing timed out after 30 seconds", f.Name)
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: nil, err: fmt.Errorf("timeout")}
			}
		}(fighter)
	}

	wg.Wait()
	close(results)
	
	app.logger.Printf("🏁 Active fighter wiki scrape completed in %v!", time.Since(startTime).Round(time.Second))
}

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

func (app *ScraperApp) runFullWikiFighterScrape() {
	startTime := time.Now()
	app.logger.Println("Starting full wiki fighter data scrape...")

	fighters, err := getAllFighters(app.db)
	if err != nil {
		app.logger.Fatalf("Error getting fighters: %v", err)
	}

	app.logger.Printf("Found %d fighters to process", len(fighters))

	maxConcurrency := 8
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 120*time.Minute)
	defer dbCancel()

	results := make(chan struct {
		fighter Fighter
		info    *scrapers.FighterExtraInfo
		err     error
	}, maxConcurrency)

	go func() {
		processedCount := 0
		startTime := time.Now()
		
		for result := range results {
			processedCount++
			
			if result.err != nil {
				app.logger.Printf("%s: ❌ Failed - %v", result.fighter.Name, result.err)
				continue
			}

			if result.info == nil {
				app.logger.Printf("%s: ❌ No data found", result.fighter.Name)
				continue
			}

			if err := updateFighterInDatabase(dbCtx, app.db, result.fighter, result.info); err != nil {
				app.logger.Printf("Failed to update fighter %s: %v", result.fighter.Name, err)
			} else {
				app.logger.Printf("%s: ✅ Updated successfully", result.fighter.Name)
			}
			
			elapsedTime := time.Since(startTime)
			timePerFighter := elapsedTime / time.Duration(processedCount)
			app.logger.Printf("Progress: %d/%d fighters processed (~%v per fighter)", 
				processedCount, len(fighters), timePerFighter.Round(time.Second))
		}
		
		app.logger.Printf("🏁 Full processing completed for %d fighters in %v", 
			processedCount, time.Since(startTime).Round(time.Second))
	}()

	fighterScraper := scrapers.NewWikiFighterScraper(app.scraperConfig)

	for _, fighter := range fighters {
		wg.Add(1)
		sem <- struct{}{}
		
		go func(f Fighter) {
			defer wg.Done()
			defer func() { <-sem }()
			
			fighterCtx, fighterCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer fighterCancel()
			
			done := make(chan bool)
			var info *scrapers.FighterExtraInfo
			var scrapeErr error
			
			go func() {
				info, scrapeErr = fighterScraper.ScrapeExtraInfo(f.Name, f.WikiURL, "", f.Wins, f.Losses)
				select {
				case <-fighterCtx.Done():
				case done <- true:
				}
			}()
			
			select {
			case <-done:
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: info, err: scrapeErr}
			case <-fighterCtx.Done():
				app.logger.Printf("%s: ⚠️ Processing timed out after 30 seconds", f.Name)
				results <- struct {
					fighter Fighter
					info    *scrapers.FighterExtraInfo
					err     error
				}{fighter: f, info: nil, err: fmt.Errorf("timeout")}
			}
		}(fighter)
	}

	wg.Wait()
	close(results)
	
	app.logger.Printf("🏁 Full wiki fighter scrape completed in %v!", time.Since(startTime).Round(time.Second))
}