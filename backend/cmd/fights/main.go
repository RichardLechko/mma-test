package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

// EventInfo holds event information from the database
type EventInfo struct {
	ID        string
	Name      string
	EventDate time.Time
	UFCURL    string
}

// FightResult tracks the processing result of a fight
type FightResult struct {
	EventName string
	FightName string
	Success   bool
	Error     error
}

// Comprehensive fighter struct combining Fighter and FighterExtraInfo
type ManualFighterData struct {
	// Core Fighter fields
	Name        string
	Nickname    string
	WeightClass string
	Status      string
	Ranking     string
	UFCID       string
	UFCURL      string
	Nationality string

	// Fighter physical attributes
	Age    int
	Height string
	Weight string
	Reach  string

	// Win methods
	KOWins  int
	SubWins int
	DecWins int

	// Extra info fields
	KOLosses      int
	SubLosses     int
	DecLosses     int
	DQLosses      int
	NoContests    int
	FightingOutOf string
}

// calculateAge calculates age from a date of birth string in MM/DD/YYYY format
func calculateAge(dob string) int {
	// Parse the DOB string
	t, err := time.Parse("01/02/2006", dob)
	if err != nil {
		return 0 // Return 0 for invalid dates
	}

	// Get current time
	now := time.Now()

	// Calculate age
	age := now.Year() - t.Year()

	// Adjust age if birthday hasn't occurred yet this year
	if now.Month() < t.Month() || (now.Month() == t.Month() && now.Day() < t.Day()) {
		age--
	}

	return age
}

// Populate manual fighters with comprehensive data
var manualFightersData = map[string]ManualFighterData{
	"Derrick Mehmen": {
		Name:          "Derrick Mehmen",
		Nickname:      "",
		WeightClass:   "Light Heavyweight",
		Status:        "Not Fighting",
		Ranking:       "Unranked",
		UFCID:         "derrick-mehmen",
		UFCURL:        "",
		Nationality:   "United States",
		Age:           calculateAge("04/15/1985"), // DOB: April 15, 1985
		Height:        "6' 3\"",
		Weight:        "205 lbs",
		Reach:         "76\"",
		KOWins:        8,
		SubWins:       4,
		DecWins:       7,
		KOLosses:      3,
		SubLosses:     2,
		DecLosses:     2,
		DQLosses:      0,
		NoContests:    0,
		FightingOutOf: "Cedar Rapids, Iowa",
	},
	"Goran Reljic": {
		Name:          "Goran Reljic",
		Nickname:      "",
		WeightClass:   "Light Heavyweight",
		Status:        "Not Fighting",
		Ranking:       "Unranked",
		UFCID:         "goran-reljic",
		UFCURL:        "",
		Nationality:   "Croatia",
		Age:           calculateAge("03/20/1983"), // DOB: March 20, 1983
		Height:        "6' 3\"",
		Weight:        "205 lbs",
		Reach:         "77\"",
		KOWins:        6,
		SubWins:       3,
		DecWins:       3,
		KOLosses:      1,
		SubLosses:     1,
		DecLosses:     2,
		DQLosses:      0,
		NoContests:    0,
		FightingOutOf: "Zadar, Croatia",
	},
	"Jason Godsey": {
		Name:          "Jason Godsey",
		Nickname:      "",
		WeightClass:   "Heavyweight",
		Status:        "Not Fighting",
		Ranking:       "Unranked",
		UFCID:         "jason-godsey",
		UFCURL:        "",
		Nationality:   "United States",
		Age:           calculateAge("02/10/1979"), // DOB: Feb 10, 1979
		Height:        "6' 0\"",
		Weight:        "230 lbs",
		Reach:         "74\"",
		KOWins:        2,
		SubWins:       1,
		DecWins:       1,
		KOLosses:      1,
		SubLosses:     1,
		DecLosses:     0,
		DQLosses:      0,
		NoContests:    0,
		FightingOutOf: "Columbus, Ohio",
	},
	"Гловер Тейшейра": {
		Name:          "Godofredo Pepey",
		Nickname:      "Pepey",
		WeightClass:   "Featherweight",
		Status:        "Not Fighting",
		Ranking:       "Unranked",
		UFCID:         "godofredo-pepey",
		UFCURL:        "",
		Nationality:   "Brazil",
		Age:           calculateAge("07/22/1987"),
		Height:        "5' 10\"",
		Weight:        "145 lbs",
		Reach:         "70\"",
		KOWins:        2,
		SubWins:       9,
		DecWins:       2,
		KOLosses:      1,
		SubLosses:     1,
		DecLosses:     3,
		DQLosses:      0,
		NoContests:    0,
		FightingOutOf: "Fortaleza, Brazil",
	},
	"Godofredo Pepey": {
		// Same data as above
		Name:          "Godofredo Pepey",
		Nickname:      "Pepey",
		WeightClass:   "Featherweight",
		Status:        "Not Fighting",
		Ranking:       "Unranked",
		UFCID:         "godofredo-pepey",
		UFCURL:        "",
		Nationality:   "Brazil",
		Age:           calculateAge("07/22/1987"),
		Height:        "5' 10\"",
		Weight:        "145 lbs",
		Reach:         "70\"",
		KOWins:        2,
		SubWins:       9,
		DecWins:       2,
		KOLosses:      1,
		SubLosses:     1,
		DecLosses:     3,
		DQLosses:      0,
		NoContests:    0,
		FightingOutOf: "Fortaleza, Brazil",
	},
	"Zelim Imadaev": {
		Name:          "Zelim Imadaev",
		Nickname:      "Borz",
		WeightClass:   "Welterweight",
		Status:        "Not Fighting",
		Ranking:       "Unranked",
		UFCID:         "zelim-imadaev",
		UFCURL:        "",
		Nationality:   "Russia",
		Age:           calculateAge("01/03/1995"), // DOB: Jan 3, 1995
		Height:        "6' 0\"",
		Weight:        "170 lbs",
		Reach:         "74\"",
		KOWins:        8,
		SubWins:       0,
		DecWins:       0,
		KOLosses:      0,
		SubLosses:     1,
		DecLosses:     2,
		DQLosses:      0,
		NoContests:    0,
		FightingOutOf: "Moscow, Russia",
	},
}

func main() {
	// Set up logging
	log.Println("🚀 Starting Fight Scraper")
	startTime := time.Now()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// Silently continue if .env not found
	}

	// Load configuration
	if err := config.LoadConfig("config/config.json"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()

	// Get events with UFC URLs from the database
	events, err := getEventsWithUFCURLs(db)
	if err != nil {
		log.Fatalf("Error fetching events: %v", err)
	}

	log.Printf("Found %d events with UFC URLs to process", len(events))

	// Create scraper with configuration
	scraperConfig := &scrapers.ScraperConfig{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
	ufcScraper := scrapers.NewUFCFightScraper(scraperConfig)

	// Create database context with longer timeout for the entire operation
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 120*time.Minute)
	defer dbCancel()

	// Set up concurrency control
	const maxConcurrentEvents = 3 // Process 3 events at a time
	sem := make(chan struct{}, maxConcurrentEvents)

	var wg sync.WaitGroup
	resultChan := make(chan FightResult, 100) // Buffer for results

	// Stats tracking
	var statsMutex sync.Mutex
	totalFightsSaved := 0
	totalEventsProcessed := 0

	// Start result processor
	go processResults(resultChan)

	// Process events concurrently
	for i, event := range events {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore slot

		go func(idx int, event EventInfo) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore slot

			log.Printf("Processing event %d/%d: %s", idx+1, len(events), event.Name)

			// Create context with timeout for this event
			eventCtx, eventCancel := context.WithTimeout(dbCtx, 5*time.Minute)
			defer eventCancel()

			// Process a single event
			fightsSaved := processEvent(eventCtx, db, ufcScraper, event, resultChan)

			// Update stats
			statsMutex.Lock()
			totalEventsProcessed++
			totalFightsSaved += fightsSaved
			statsMutex.Unlock()

			// Add a small delay between requests to avoid rate limiting
			time.Sleep(1 * time.Second)
		}(i, event)
	}

	// Wait for all events to be processed
	wg.Wait()
	close(resultChan)

	// Wait a moment for the result processor to finish
	time.Sleep(100 * time.Millisecond)

	log.Printf("🏁 Scraping completed in %v! Processed %d/%d events, saved %d fights total.",
		time.Since(startTime).Round(time.Second), totalEventsProcessed, len(events), totalFightsSaved)
}

// connectToDatabase establishes a connection to the database
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// getEventsWithUFCURLs retrieves events with UFC URLs from the database
func getEventsWithUFCURLs(db *sql.DB) ([]EventInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get events with UFC URLs from the database
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, event_date, ufc_url 
		FROM events 
		WHERE ufc_url IS NOT NULL AND ufc_url != '' 
		ORDER BY event_date DESC`)
	if err != nil {
		return nil, fmt.Errorf("error querying events: %w", err)
	}
	defer rows.Close()

	var events []EventInfo

	for rows.Next() {
		var event EventInfo
		if err := rows.Scan(&event.ID, &event.Name, &event.EventDate, &event.UFCURL); err != nil {
			continue
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event rows: %w", err)
	}

	return events, nil
}

// processResults handles the result channel and logs results
func processResults(resultChan <-chan FightResult) {
	// Track events we've seen
	seenEvents := make(map[string]int)

	for result := range resultChan {
		// Update event stats
		if _, exists := seenEvents[result.EventName]; !exists {
			seenEvents[result.EventName] = 0
		}
		seenEvents[result.EventName]++

		// Log errors
		if !result.Success {
			log.Printf("❌ %s: %v", result.FightName, result.Error)
		}
	}
}

// processEvent handles a single event, scraping and saving fights
func processEvent(ctx context.Context, db *sql.DB, scraper *scrapers.UFCFightScraper,
	event EventInfo, resultChan chan<- FightResult) int {

	// Scrape fights from the UFC URL
	ufcFights, err := scraper.ScrapeFightsWithContext(ctx, event.UFCURL)
	if err != nil || len(ufcFights) == 0 {
		resultChan <- FightResult{
			EventName: event.Name,
			FightName: "Event scraping",
			Success:   false,
			Error:     fmt.Errorf("failed to scrape event from URL %s: %w", event.UFCURL, err),
		}
		return 0
	}

	// Use a wait group to process fights concurrently
	var fightWg sync.WaitGroup
	fightSem := make(chan struct{}, 5) // Process up to 5 fights concurrently

	var fightsMutex sync.Mutex
	fightsSavedForEvent := 0

	// Process each fight - the order from the scraper already has the main event first
	for j, fight := range ufcFights {
		fightWg.Add(1)
		fightSem <- struct{}{} // Acquire fight semaphore

		go func(idx int, fight scrapers.UFCScrapedFight) {
			defer fightWg.Done()
			defer func() { <-fightSem }() // Release fight semaphore

			// Process a single fight
			fightName := fmt.Sprintf("%s vs %s", fight.Fighter1Name, fight.Fighter2Name)

			fighter1ID, err := findFighterId(ctx, db, fight.Fighter1Name, fight.Fighter1LastName, event.Name)
			if err != nil {
				resultChan <- FightResult{
					EventName: event.Name,
					FightName: fightName,
					Success:   false,
					Error:     fmt.Errorf("fighter '%s' not found: %w", fight.Fighter1Name, err),
				}
				return
			}

			fighter2ID, err := findFighterId(ctx, db, fight.Fighter2Name, fight.Fighter2LastName, event.Name)
			if err != nil {
				resultChan <- FightResult{
					EventName: event.Name,
					FightName: fightName,
					Success:   false,
					Error:     fmt.Errorf("fighter '%s' not found: %w", fight.Fighter2Name, err),
				}
				return
			}

			// Determine winner ID if available
			var winnerID *string
			if fight.Fighter1Result == "Win" {
				winnerID = &fighter1ID
			} else if fight.Fighter2Result == "Win" {
				winnerID = &fighter2ID
			}

			// Determine championship status
			fighter1WasChampion := strings.Contains(strings.ToLower(fight.Fighter1Rank), "c")
			fighter2WasChampion := strings.Contains(strings.ToLower(fight.Fighter2Rank), "c")

			// Extract round as integer
			var resultRound *int
			if fight.Round != "" {
				roundInt, err := strconv.Atoi(strings.TrimSpace(fight.Round))
				if err == nil {
					resultRound = &roundInt
				}
			}

			// Process method details
			var resultMethod, resultMethodDetails string
			methodParts := strings.Split(fight.Method, " - ")
			if len(methodParts) > 0 {
				resultMethod = methodParts[0]
				if len(methodParts) > 1 {
					resultMethodDetails = methodParts[1]
				}
			}

			// Convert ranks to just the number
			fighter1Rank := stripRankPrefix(fight.Fighter1Rank)
			fighter2Rank := stripRankPrefix(fight.Fighter2Rank)

			// Create query
			query := `
			INSERT INTO fights (
				event_id, fighter1_id, fighter2_id, fighter1_name, fighter2_name, 
				weight_class, is_main_event, fight_order, 
				fighter1_was_champion, fighter2_was_champion, was_title_fight,
				winner_id, result_method, result_method_details, result_round, result_time,
				fighter1_rank, fighter2_rank,
				created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
			)
			ON CONFLICT (event_id, fighter1_id, fighter2_id) 
			DO UPDATE SET
				fighter1_name = EXCLUDED.fighter1_name,
				fighter2_name = EXCLUDED.fighter2_name,
				weight_class = EXCLUDED.weight_class,
				is_main_event = EXCLUDED.is_main_event,
				fight_order = EXCLUDED.fight_order,
				fighter1_was_champion = EXCLUDED.fighter1_was_champion,
				fighter2_was_champion = EXCLUDED.fighter2_was_champion,
				was_title_fight = EXCLUDED.was_title_fight,
				winner_id = EXCLUDED.winner_id,
				result_method = EXCLUDED.result_method,
				result_method_details = EXCLUDED.result_method_details,
				result_round = EXCLUDED.result_round,
				result_time = EXCLUDED.result_time,
				fighter1_rank = EXCLUDED.fighter1_rank,
				fighter2_rank = EXCLUDED.fighter2_rank,
				updated_at = EXCLUDED.updated_at
			RETURNING id`

			now := time.Now()
			var fightID string

			// For fight_order, use index+1 directly - this preserves the order from the UFC website
			err = db.QueryRowContext(ctx, query,
				event.ID,            // $1
				fighter1ID,          // $2
				fighter2ID,          // $3
				fight.Fighter1Name,  // $4
				fight.Fighter2Name,  // $5
				fight.WeightClass,   // $6
				fight.IsMainEvent,   // $7
				idx+1,               // $8 - fight_order (preserves UFC website order)
				fighter1WasChampion, // $9
				fighter2WasChampion, // $10
				fight.IsTitleFight,  // $11
				winnerID,            // $12
				resultMethod,        // $13
				resultMethodDetails, // $14
				resultRound,         // $15
				fight.Time,          // $16
				fighter1Rank,        // $17
				fighter2Rank,        // $18
				now,                 // $19
				now,                 // $20
			).Scan(&fightID)

			if err != nil {
				resultChan <- FightResult{
					EventName: event.Name,
					FightName: fightName,
					Success:   false,
					Error:     fmt.Errorf("failed to save fight: %w", err),
				}
				return
			}

			// Update fight count
			fightsMutex.Lock()
			fightsSavedForEvent++
			fightsMutex.Unlock()

			resultChan <- FightResult{
				EventName: event.Name,
				FightName: fightName,
				Success:   true,
			}
		}(j, fight)
	}

	// Wait for all fights to be processed
	fightWg.Wait()

	log.Printf("✅ Saved %d fights for event '%s'", fightsSavedForEvent, event.Name)
	return fightsSavedForEvent
}

func findFighterId(ctx context.Context, db *sql.DB, fighterName, fighterLastName, eventName string) (string, error) {
	// Check if this is a special case
	mappedName := mapFighterName(fighterName)

	if mappedName == "MANUAL" {
		// First try the original fighter name
		manualData, exists := manualFightersData[fighterName]

		// If not found by original name, check if the mapped name exists
		if !exists && fighterName != mappedName {
			manualData, exists = manualFightersData[mappedName]
		}

		if exists {
			// Convert to Fighter struct for insertion
			fighter := scrapers.Fighter{
				Name:        manualData.Name,
				Nickname:    manualData.Nickname,
				WeightClass: manualData.WeightClass,
				Status:      manualData.Status,
				Ranking:     manualData.Ranking,
				UFCID:       manualData.UFCID,
				UFCURL:      manualData.UFCURL,
				Nationality: manualData.Nationality,
				Age:         manualData.Age,
				Height:      manualData.Height,
				Weight:      manualData.Weight,
				Reach:       manualData.Reach,
				KOWins:      manualData.KOWins,
				SubWins:     manualData.SubWins,
				DecWins:     manualData.DecWins,
			}

			// Try to insert the fighter
			err := scrapers.InsertFighter(db, &fighter)
			if err != nil {
				return "", fmt.Errorf("failed to manually insert fighter '%s': %w", fighterName, err)
			}

			// Now try to get the ID of the newly inserted fighter
			var fighterId string
			err = db.QueryRowContext(ctx, "SELECT id FROM fighters WHERE ufc_id = $1", fighter.UFCID).Scan(&fighterId)
			if err != nil {
				return "", fmt.Errorf("fighter '%s' was inserted but ID retrieval failed: %w", fighterName, err)
			}

			return fighterId, nil
		}

		return "", fmt.Errorf("fighter '%s' requires manual entry but data not provided", fighterName)
	}

	// Handle DETOUR case - direct URL lookup
	if mappedName == "DETOUR" {
		// Map of fighter names to their direct UFC IDs from URLs
		directUrlMap := map[string]string{
			"Feng Xiaocan":            "feng-xiaocan",
			"Kiru Sahota":             "kiru-singh-sahota",
			"Marcio Alexandre Junior": "marcio-alexandre-junior",
			"Virgil Zwicker":          "virgil-zwicker",
		}

		if ufcUrlId, exists := directUrlMap[fighterName]; exists {
			// Try to find fighter by their URL ID
			var fighterId string
			err := db.QueryRowContext(ctx, "SELECT id FROM fighters WHERE ufc_id = $1", ufcUrlId).Scan(&fighterId)
			if err == nil {
				return fighterId, nil
			}

			// If not found, insert the fighter directly into the database
			// Notice we're inserting directly using the schema columns instead of using InsertFighter
			now := time.Now()

			// Parse record into components (using 0-0-0 as default)
			wins, losses, draws := 0, 0, 0

			insertQuery := `
            INSERT INTO fighters 
            (name, nickname, weight_class, status, rank, ufc_id, ufc_url, 
             nationality, age, height, weight, reach, 
             wins, losses, draws, no_contests,
             ko_wins, sub_wins, dec_wins, 
             loss_by_ko, loss_by_sub, loss_by_dec, loss_by_dq,
             created_at, updated_at) 
            VALUES 
            ($1, $2, $3, $4, $5, $6, $7, 
             $8, $9, $10, $11, $12, 
             $13, $14, $15, $16, 
             $17, $18, $19, 
             $20, $21, $22, $23, 
             $24, $25)
            RETURNING id`

			err = db.QueryRowContext(ctx, insertQuery,
				fighterName, // $1 name
				"",          // $2 nickname
				"Unknown",   // $3 weight_class
				"Active",    // $4 status
				"Unranked",  // $5 rank
				ufcUrlId,    // $6 ufc_id
				"https://www.ufc.com/athlete/"+ufcUrlId, // $7 ufc_url
				"",     // $8 nationality
				0,      // $9 age
				"",     // $10 height
				"",     // $11 weight
				"",     // $12 reach
				wins,   // $13 wins
				losses, // $14 losses
				draws,  // $15 draws
				0,      // $16 no_contests
				0,      // $17 ko_wins
				0,      // $18 sub_wins
				0,      // $19 dec_wins
				0,      // $20 loss_by_ko
				0,      // $21 loss_by_sub
				0,      // $22 loss_by_dec
				0,      // $23 loss_by_dq
				now,    // $24 created_at
				now,    // $25 updated_at
			).Scan(&fighterId)

			if err != nil {
				return "", fmt.Errorf("failed to insert detour fighter '%s': %w", fighterName, err)
			}

			return fighterId, nil
		}
	}

	// Skip case
	if mappedName == "SKIP" {
		return "", fmt.Errorf("fighter '%s' skipped (known edge case) for event '%s'", fighterName, eventName)
	}

	var fighterId string
	var err error

	// Try exact match with original name
	err = db.QueryRowContext(ctx, "SELECT id FROM fighters WHERE LOWER(name) = LOWER($1)", mappedName).Scan(&fighterId)
	if err == nil {
		return fighterId, nil
	}

	// Try with normalized name
	normalizedName := normalizeAccents(mappedName)
	err = db.QueryRowContext(ctx, "SELECT id FROM fighters WHERE LOWER(name) = LOWER($1)", normalizedName).Scan(&fighterId)
	if err == nil {
		return fighterId, nil
	}

	// Try with last name if it's long enough
	if len(fighterLastName) > 3 {
		normalizedLastName := normalizeAccents(fighterLastName)
		err = db.QueryRowContext(ctx, "SELECT id FROM fighters WHERE LOWER(name) LIKE '%' || LOWER($1) || '%'", normalizedLastName).Scan(&fighterId)
		if err == nil {
			return fighterId, nil
		}
	}

	return "", fmt.Errorf("fighter '%s' not found in database for event '%s'", fighterName, eventName)
}

// Helper function to strip rank prefix (e.g., "#3" -> "3")
func stripRankPrefix(rank string) string {
	rank = strings.TrimSpace(rank)
	if rank == "" {
		return ""
	}

	// Remove leading characters that are not digits
	re := regexp.MustCompile(`[^\d]*(\d+)`)
	matches := re.FindStringSubmatch(rank)
	if len(matches) > 1 {
		return matches[1]
	}

	// If "C" or "Champion", return special value
	if strings.Contains(strings.ToLower(rank), "c") {
		return "C"
	}

	return rank
}

// normalizeAccents replaces accented characters with their non-accented equivalents
func normalizeAccents(name string) string {
	// Create a map for accented character replacements
	replacements := map[string]string{
		// Lowercase
		"á": "a", "à": "a", "â": "a", "ä": "a", "ã": "a", "å": "a", "ą": "a", "ă": "a", "ā": "a",
		"é": "e", "è": "e", "ê": "e", "ë": "e", "ę": "e", "ė": "e", "ě": "e", "ē": "e",
		"í": "i", "ì": "i", "î": "i", "ï": "i", "ı": "i", "ī": "i",
		"ó": "o", "ò": "o", "ô": "o", "ö": "o", "õ": "o", "ø": "o", "ő": "o", "ō": "o", "ơ": "o",
		"ú": "u", "ù": "u", "û": "u", "ü": "u", "ū": "u", "ů": "u", "ű": "u", "ų": "u", "ư": "u",
		"ý": "y", "ÿ": "y", "ŷ": "y",
		"ç": "c", "č": "c", "ć": "c", "ĉ": "c",
		"ñ": "n", "ń": "n", "ň": "n", "ņ": "n",
		"ş": "s", "ś": "s", "š": "s", "ș": "s", "ŝ": "s",
		"ž": "z", "ź": "z", "ż": "z",
		"ł": "l", "ĺ": "l", "ļ": "l", "ľ": "l",
		"ť": "t", "ț": "t", "ţ": "t",
		"ď": "d", "đ": "d",
		"ř": "r", "ŕ": "r", "ŗ": "r",
		"ğ": "g", "ĝ": "g", "ģ": "g", "ġ": "g",
		"ĵ": "j", "ķ": "k", "ĥ": "h", "ħ": "h",

		// Uppercase
		"Á": "A", "À": "A", "Â": "A", "Ä": "A", "Ã": "A", "Å": "A", "Ą": "A", "Ă": "A", "Ā": "A",
		"É": "E", "È": "E", "Ê": "E", "Ë": "E", "Ę": "E", "Ė": "E", "Ě": "E", "Ē": "E",
		"Í": "I", "Ì": "I", "Î": "I", "Ï": "I", "İ": "I", "Ī": "I",
		"Ó": "O", "Ò": "O", "Ô": "O", "Ö": "O", "Õ": "O", "Ø": "O", "Ő": "O", "Ō": "O", "Ơ": "O",
		"Ú": "U", "Ù": "U", "Û": "U", "Ü": "U", "Ū": "U", "Ů": "U", "Ű": "U", "Ų": "U", "Ư": "U",
		"Ý": "Y", "Ÿ": "Y", "Ŷ": "Y",
		"Ç": "C", "Č": "C", "Ć": "C", "Ĉ": "C",
		"Ñ": "N", "Ń": "N", "Ň": "N", "Ņ": "N",
		"Ş": "S", "Ś": "S", "Š": "S", "Ș": "S", "Ŝ": "S",
		"Ž": "Z", "Ź": "Z", "Ż": "Z",
		"Ł": "L", "Ĺ": "L", "Ļ": "L", "Ľ": "L",
		"Ť": "T", "Ț": "T", "Ţ": "T",
		"Ď": "D", "Đ": "D",
		"Ř": "R", "Ŕ": "R", "Ŗ": "R",
		"Ğ": "G", "Ĝ": "G", "Ģ": "G", "Ġ": "G",
		"Ĵ": "J", "Ķ": "K", "Ĥ": "H", "Ħ": "H",
	}

	// Apply all replacements
	for accented, nonAccented := range replacements {
		name = strings.ReplaceAll(name, accented, nonAccented)
	}

	return name
}

func mapFighterName(name string) string {
	knownMappings := map[string]string{
		"Гловер Тейшейра": "Godofredo Pepey",
		"Cesar Martucci":  "Cesar Marscucci",

		"Derrick Mehmen":          "MANUAL",
		"Feng Xiaocan":            "DETOUR",
		"Goran Reljic":            "MANUAL",
		"Godofredo Pepey":         "MANUAL",
		"Jason Godsey":            "MANUAL",
		"Kiru Sahota":             "DETOUR",
		"Marcio Alexandre Junior": "DETOUR",
		"Virgil Zwicker":          "DETOUR",
		"Zelim Imadaev":           "MANUAL",
	}

	mappedName, exists := knownMappings[name]
	if exists {
		if secondMappedName, secondExists := knownMappings[mappedName]; secondExists {
			return secondMappedName
		}
		return mappedName
	}

	normalizedName := normalizeAccents(name)
	if mappedName, exists := knownMappings[normalizedName]; exists {
		if secondMappedName, secondExists := knownMappings[mappedName]; secondExists {
			return secondMappedName
		}
		return mappedName
	}

	return name
}
