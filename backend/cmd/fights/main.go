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

	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Get events with UFC URLs from the database
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, event_date, ufc_url 
		FROM events 
		WHERE ufc_url IS NOT NULL AND ufc_url != '' 
		ORDER BY event_date DESC`)
	if err != nil {
		log.Fatalf("Error querying events: %v", err)
	}
	defer rows.Close()

	var events []struct {
		ID        string
		Name      string
		EventDate time.Time
		UFCURL    string
	}

	for rows.Next() {
		var event struct {
			ID        string
			Name      string
			EventDate time.Time
			UFCURL    string
		}
		if err := rows.Scan(&event.ID, &event.Name, &event.EventDate, &event.UFCURL); err != nil {
			continue
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating event rows: %v", err)
	}

	ufcScraper := scrapers.NewUFCFightScraper()
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 60*time.Minute) // Increased timeout
	defer dbCancel()

	totalFightsSaved := 0
	totalEventsProcessed := 0

	// Process each event
	for i, event := range events {
		log.Printf("Processing event %d/%d: %s", i+1, len(events), event.Name)
		
		// Scrape fights from the UFC URL
		ufcFights, err := ufcScraper.ScrapeFights(event.UFCURL)
		if err != nil || len(ufcFights) == 0 {
			log.Printf("Failed to scrape event %s from URL %s: %v", event.Name, event.UFCURL, err)
			continue
		}

		fightsSavedForEvent := 0

		// Process each fight
		for j, fight := range ufcFights {
			// For fighter1:
			var fighter1ID string

			// Check if this is a special case that should be skipped
			mappedName1 := mapFighterName(fight.Fighter1Name)
			if mappedName1 == "SKIP" {
				log.Printf("Fighter '%s' skipped (known edge case) for event '%s'", fight.Fighter1Name, event.Name)
				continue
			}

			// Try exact match with original name
			err = db.QueryRowContext(dbCtx, "SELECT id FROM fighters WHERE LOWER(name) = LOWER($1)", mappedName1).Scan(&fighter1ID)
			if err != nil {
				// Try with normalized name
				normalizedName := normalizeAccents(mappedName1)
				err = db.QueryRowContext(dbCtx, "SELECT id FROM fighters WHERE LOWER(name) = LOWER($1)", normalizedName).Scan(&fighter1ID)

				// If still not found, try with last name
				if err != nil && len(fight.Fighter1LastName) > 3 {
					normalizedLastName := normalizeAccents(fight.Fighter1LastName)
					err = db.QueryRowContext(dbCtx, "SELECT id FROM fighters WHERE LOWER(name) LIKE '%' || LOWER($1) || '%'", normalizedLastName).Scan(&fighter1ID)
					if err != nil {
						log.Printf("Fighter '%s' not found in database for event '%s', skipping fight", fight.Fighter1Name, event.Name)
						continue
					}
				} else if err != nil {
					log.Printf("Fighter '%s' not found in database for event '%s', skipping fight", fight.Fighter1Name, event.Name)
					continue
				}
			}

			// For fighter2:
			var fighter2ID string

			// Check if this is a special case that should be skipped
			mappedName2 := mapFighterName(fight.Fighter2Name)
			if mappedName2 == "SKIP" {
				log.Printf("Fighter '%s' skipped (known edge case) for event '%s'", fight.Fighter2Name, event.Name)
				continue
			}

			// Try exact match with original name
			err = db.QueryRowContext(dbCtx, "SELECT id FROM fighters WHERE LOWER(name) = LOWER($1)", mappedName2).Scan(&fighter2ID)
			if err != nil {
				// Try with normalized name
				normalizedName := normalizeAccents(mappedName2)
				err = db.QueryRowContext(dbCtx, "SELECT id FROM fighters WHERE LOWER(name) = LOWER($1)", normalizedName).Scan(&fighter2ID)

				// If still not found, try with last name
				if err != nil && len(fight.Fighter2LastName) > 3 {
					normalizedLastName := normalizeAccents(fight.Fighter2LastName)
					err = db.QueryRowContext(dbCtx, "SELECT id FROM fighters WHERE LOWER(name) LIKE '%' || LOWER($1) || '%'", normalizedLastName).Scan(&fighter2ID)
					if err != nil {
						log.Printf("Fighter '%s' not found in database for event '%s', skipping fight", fight.Fighter2Name, event.Name)
						continue
					}
				} else if err != nil {
					log.Printf("Fighter '%s' not found in database for event '%s', skipping fight", fight.Fighter2Name, event.Name)
					continue
				}
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

			err = db.QueryRowContext(dbCtx, query,
				event.ID,            // $1
				fighter1ID,          // $2
				fighter2ID,          // $3
				fight.Fighter1Name,  // $4
				fight.Fighter2Name,  // $5
				fight.WeightClass,   // $6
				fight.IsMainEvent,   // $7
				j+1,                 // $8 - fight_order
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
				log.Printf("Failed to save fight %s vs %s: %v", fight.Fighter1Name, fight.Fighter2Name, err)
				continue
			}

			fightsSavedForEvent++
			totalFightsSaved++
		}

		log.Printf("Saved %d fights for event '%s'", fightsSavedForEvent, event.Name)
		totalEventsProcessed++

		// Add a small delay between requests to avoid rate limiting
		time.Sleep(1 * time.Second)
	}

	log.Printf("Scraping completed! Processed %d/%d events, saved %d fights total.", 
		totalEventsProcessed, len(events), totalFightsSaved)

	// Removed the backfill fighters section as requested
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

// mapFighterName handles known edge cases and fighter name variations
func mapFighterName(name string) string {
	// Commenting out the hard-coded mappings as requested
	/*
	knownMappings := map[string]string{
		"Alberto Uda":             "Alberto Pereira",
		"Alex Stiebling":          "Alex Steibling",
		"Alexey Oleynik":          "Aleksei Oleinik",
		"Antonio dos Santos Jr.":  "Antonio dos Santos",
		"Antônio dos Santos Jr.":  "Antonio dos Santos",
		"Ariane Lipski":           "Ariane da Silva",
		"Carlo Pedersoli Jr.":     "Carlo Pedersoli",
		"Choi Doo-ho":             "Dooho Choi",
		"Chris Liguori":           "Chris Ligouri",
		"Dmitry Smolyakov":        "Dmitrii Smoliakov",
		"Гловер Тейшейра":         "Godofredo Pepey",
		"Jose Maria Tome":         "Jose Maria",
		"Joanne Calderwood":       "Joanne Wood",
		"Kang Kyung-ho":           "Kyung Ho Kang",
		"Katlyn Chookagian":       "Katlyn Cerminara",
		"Katsuhisa Fujii":         "Katsuhisa Fuji",
		"Kim Dong-hyun":           "Dong Hyun Kim",
		"Lim Hyun-gyu":            "Hyun Gyu Lim",
		"Luiz Dutra Jr.":          "Luiz Dutra",
		"Marcio Alexandre Jr.":    "Marcio Alexandre",
		"Márcio Alexandre Jr.":    "Marcio Alexandre",
		"Marcos Rosa Mariano":     "Marcos Rosa",
		"Markus Perez Echeimberg": "Markus Perez",
		"Mostapha al-Turk":        "Mostapha Al Turk",
		"Nina Ansaroff":           "Nina Nunes",
		"Seo Hee Ham":             "Seohee Ham",
		"Tony Petarra":            "Tony Peterra",
		"Veronica Macedo":         "Veronica Hardy",
		"Zhumabek Tursyn":         "Jumabieke Tuerxun",
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
	*/

	// Just return the original name since mappings are commented out
	return name
}