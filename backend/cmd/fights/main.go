package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mma-scheduler/config"
	"mma-scheduler/pkg/scrapers"

	"github.com/PuerkitoBio/goquery"
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

	rows, err := db.QueryContext(ctx, `
SELECT id, name, event_date, wiki_url FROM events 
WHERE wiki_url IS NOT NULL AND wiki_url != '' 

AND (
    name LIKE 'UFC %' 
    OR name LIKE 'UFC: %'
    OR name LIKE 'UFC Japan%'
    OR name LIKE 'UFC Brazil%'
    OR name = 'UFC'
)
AND name NOT IN (
    'UFC 1: The Beginning', 'UFC 2: No Way Out', 'UFC 3: The American Dream',
    'UFC 4: Revenge of the Warriors', 'UFC 5: The Return of the Beast',
    'UFC 6: Clash of the Titans', 'UFC 7: The Brawl in Buffalo',
    'UFC: The Ultimate Ultimate', 'UFC 8: David vs. Goliath', 'UFC 10: The Tournament',
    'UFC 11: The Proving Ground', 'UFC: The Ultimate Ultimate 2',
    'UFC 12: Judgement Day', 'UFC 13: The Ultimate Force', 'UFC 14: Showdown',
    'UFC 15: Collision Course', 'UFC Japan: Ultimate Japan',
    'UFC 16: Battle in the Bayou', 'UFC 17: Redemption', 'UFC 23: Ultimate Japan 2'
)
ORDER BY event_date DESC`)
	if err != nil {
		log.Fatalf("Error querying events: %v", err)
	}
	defer rows.Close()

	var events []struct {
		ID        string
		Name      string
		EventDate time.Time
		WikiURL   string
	}

	for rows.Next() {
		var event struct {
			ID        string
			Name      string
			EventDate time.Time
			WikiURL   string
		}
		if err := rows.Scan(&event.ID, &event.Name, &event.EventDate, &event.WikiURL); err != nil {
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

	for _, event := range events {
		// Only process UFC events
		if !strings.Contains(strings.ToLower(event.Name), "ufc") {
			continue
		}

		var ufcFights []scrapers.UFCScrapedFight
		var err error
		var successfulScrape bool

		// Attempt 1: Try with UFC event number
		if eventNumber := extractUFCEventNumber(event.Name); eventNumber != "" {
			ufcURL := fmt.Sprintf("https://www.ufc.com/event/ufc-%s", eventNumber)

			ufcFights, err = ufcScraper.ScrapeFights(ufcURL)
			if err == nil && len(ufcFights) > 0 {
				successfulScrape = true
			}
		}

		// Attempt 2: Try with date format (both padded and unpadded day)
		if !successfulScrape {
			date := event.EventDate.UTC()
			month := strings.ToLower(date.Month().String())
			day := date.Day()
			year := date.Year()

			// Try exact event date with different formats
			// Format 1: ufc-fight-night-january-20-2024
			ufcURL := fmt.Sprintf("https://www.ufc.com/event/ufc-fight-night-%s-%d-%d", month, day, year)
			ufcFights, err = ufcScraper.ScrapeFights(ufcURL)

			// Format 2: ufc-fight-night-january-20-2024-location
			if err != nil || len(ufcFights) == 0 {
				// Extract location from event name if available
				nameParts := strings.Split(event.Name, ":")
				if len(nameParts) > 1 {
					location := strings.TrimSpace(nameParts[1])
					if location != "" {
						location = strings.ToLower(location)
						location = strings.ReplaceAll(location, " ", "-")
						ufcURL = fmt.Sprintf("https://www.ufc.com/event/ufc-fight-night-%s-%d-%d-%s", month, day, year, location)
						ufcFights, err = ufcScraper.ScrapeFights(ufcURL)
					}
				}
			}

			// Try with padded day (e.g., 01, 02)
			if err != nil || len(ufcFights) == 0 {
				ufcURL = fmt.Sprintf("https://www.ufc.com/event/ufc-fight-night-%s-%02d-%d", month, day, year)
				ufcFights, err = ufcScraper.ScrapeFights(ufcURL)
			}

			// Try adjacent days if needed
			if err != nil || len(ufcFights) == 0 {
				// Try the day before with both formats
				yesterday := date.AddDate(0, 0, -1)
				month = strings.ToLower(yesterday.Month().String())
				day = yesterday.Day()
				year = yesterday.Year()

				// Padded and unpadded
				ufcURL = fmt.Sprintf("https://www.ufc.com/event/ufc-fight-night-%s-%02d-%d", month, day, year)
				ufcFights, err = ufcScraper.ScrapeFights(ufcURL)

				if err != nil || len(ufcFights) == 0 {
					ufcURL = fmt.Sprintf("https://www.ufc.com/event/ufc-fight-night-%s-%d-%d", month, day, year)
					ufcFights, err = ufcScraper.ScrapeFights(ufcURL)
				}

				// Try the day after with both formats
				if err != nil || len(ufcFights) == 0 {
					tomorrow := date.AddDate(0, 0, 1)
					month = strings.ToLower(tomorrow.Month().String())
					day = tomorrow.Day()
					year = tomorrow.Year()

					ufcURL = fmt.Sprintf("https://www.ufc.com/event/ufc-fight-night-%s-%02d-%d", month, day, year)
					ufcFights, err = ufcScraper.ScrapeFights(ufcURL)

					if err != nil || len(ufcFights) == 0 {
						ufcURL = fmt.Sprintf("https://www.ufc.com/event/ufc-fight-night-%s-%d-%d", month, day, year)
						ufcFights, err = ufcScraper.ScrapeFights(ufcURL)
					}
				}
			}

			// Try with 'ufc-' prefix instead of 'ufc-fight-night-'
			if err != nil || len(ufcFights) == 0 {
				month := strings.ToLower(event.EventDate.Month().String())
				day := event.EventDate.Day()
				year := event.EventDate.Year()

				ufcURL = fmt.Sprintf("https://www.ufc.com/event/ufc-%s-%d-%d", month, day, year)
				ufcFights, err = ufcScraper.ScrapeFights(ufcURL)
			}

			if err == nil && len(ufcFights) > 0 {
				successfulScrape = true
			}
		}

		// Then try the early UFC scraper for non-tournament events
		if !successfulScrape {
			earlyUFCFights, isEarlyEvent := tryEarlyUFCEvent(event)
			if isEarlyEvent && len(earlyUFCFights) > 0 {
				log.Printf("Event '%s': Using special early UFC event scraping", event.Name)
				ufcFights = earlyUFCFights
				successfulScrape = true
			}
		}

		if !successfulScrape && event.WikiURL != "" {
			wikiURL := event.WikiURL

			// Fetch the Wikipedia page
			resp, err := http.Get(wikiURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				doc, err := goquery.NewDocumentFromReader(resp.Body)
				resp.Body.Close()

				if err == nil {
					// Extract fights from the Wikipedia page
					wikiUFCFights := extractFightsFromWikipedia(doc)
					if len(wikiUFCFights) > 0 {
						log.Printf("Event '%s': Falling back to Wikipedia data as UFC site failed", event.Name)
						ufcFights = wikiUFCFights
						successfulScrape = true
					}
				}
			}
		}

		if !successfulScrape || len(ufcFights) == 0 {
			log.Printf("Failed to scrape event %s: No fights found", event.Name)
			continue
		}

		for j, fight := range ufcFights {
			// For fighter1:
			var fighter1ID string

			// Check if this is a special case that should be skipped
			mappedName1 := mapFighterName(fight.Fighter1Name)

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

			totalFightsSaved++
		}

		time.Sleep(1 * time.Second)
	}

	log.Println("Starting backfill process for fighters with missing fight data...")
	backfillFighters := []string{
		"Alexander Volkanovski",
		"Diego Lopes",
		// Add any other fighters with missing fights here
	}

	// Process each fighter
	for _, fighterName := range backfillFighters {
		err := scrapers.BackfillFighterHistory(db, fighterName)
		if err != nil {
			log.Printf("Error backfilling %s: %v", fighterName, err)
		} else {
			log.Printf("Successfully backfilled %s's fight history", fighterName)
		}
	}
	log.Printf("Scraping completed! Saved %d fights total.", totalFightsSaved)
}

// Extract UFC event number from event name
func extractUFCEventNumber(name string) string {
	re := regexp.MustCompile(`UFC\s*-?\s*(\d{3})`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Extract fights from Wikipedia HTML
func extractFightsFromWikipedia(doc *goquery.Document) []scrapers.UFCScrapedFight {
	var fights []scrapers.UFCScrapedFight

	// Find the main fight table - this assumes the structure in the sample you provided
	doc.Find("table.toccolours").Each(func(i int, table *goquery.Selection) {
		var inFightSection bool = false
		var isMainCard bool = false

		// Process each row in the table
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			// Check if this is a section header (Main Card, Preliminary Card)
			headerText := row.Find("th[colspan]").First().Text()

			if strings.Contains(strings.ToLower(headerText), "main card") {
				inFightSection = true
				isMainCard = true
				return // Skip the header row
			} else if strings.Contains(strings.ToLower(headerText), "preliminary card") {
				inFightSection = true
				isMainCard = false
				return // Skip the header row
			}

			// Skip if not in a fight section or if it's the column header row
			if !inFightSection || row.Find("th").Length() > 0 {
				return
			}

			// Extract fighter names, weight class and result
			cells := row.Find("td")
			if cells.Length() < 4 {
				return
			}

			weightClass := strings.TrimSpace(cells.Eq(0).Text())
			if weightClass == "" {
				return
			}

			// Check if it's a title fight
			isTitleFight := strings.Contains(strings.ToLower(weightClass), "title")

			// Extract fighter names - try the <a> tag first, if it exists
			fighter1Element := cells.Eq(1).Find("a").First()
			fighter2Element := cells.Eq(3).Find("a").First()

			var fighter1Name, fighter2Name string

			// If <a> tag exists, use its text
			if fighter1Element.Length() > 0 {
				fighter1Name = strings.TrimSpace(fighter1Element.Text())
			} else {
				// Otherwise use the cell text directly
				fighter1Name = strings.TrimSpace(cells.Eq(1).Text())
			}

			if fighter2Element.Length() > 0 {
				fighter2Name = strings.TrimSpace(fighter2Element.Text())
			} else {
				fighter2Name = strings.TrimSpace(cells.Eq(3).Text())
			}

			// Skip if either fighter name is missing
			if fighter1Name == "" || fighter2Name == "" {
				return
			}

			// Clean fighter names - remove any footnote references like [1]
			re := regexp.MustCompile(`\[\d+\]`)
			fighter1Name = strings.TrimSpace(re.ReplaceAllString(fighter1Name, ""))
			fighter2Name = strings.TrimSpace(re.ReplaceAllString(fighter2Name, ""))

			// Extract last names for better matching
			fighter1LastName := extractLastName(fighter1Name)
			fighter2LastName := extractLastName(fighter2Name)

			// Extract result method and round if available
			var method, round, time string
			if cells.Length() >= 7 {
				method = strings.TrimSpace(cells.Eq(4).Text())
				round = strings.TrimSpace(cells.Eq(5).Text())
				time = strings.TrimSpace(cells.Eq(6).Text())
			} else if cells.Length() >= 5 {
				// Some tables have fewer columns
				method = strings.TrimSpace(cells.Eq(4).Text())
			}

			// Determine winner (Wiki tables usually format with "def." in the middle)
			var fighter1Result, fighter2Result string
			resultText := strings.TrimSpace(cells.Eq(2).Text())
			if strings.Contains(resultText, "def.") {
				fighter1Result = "Win"
				fighter2Result = "Loss"
			} else if strings.Contains(resultText, "vs.") || strings.Contains(resultText, "vs") {
				// No result yet or draw
				fighter1Result = ""
				fighter2Result = ""
			}

			// Create fight struct
			fight := scrapers.UFCScrapedFight{
				Fighter1Name:     fighter1Name,
				Fighter2Name:     fighter2Name,
				Fighter1LastName: fighter1LastName,
				Fighter2LastName: fighter2LastName,
				Fighter1Result:   fighter1Result,
				Fighter2Result:   fighter2Result,
				WeightClass:      weightClass,
				IsMainEvent:      j == 2 && isMainCard, // First fight in main card is often main event
				IsTitleFight:     isTitleFight,
				Method:           method,
				Round:            round,
				Time:             time,
			}

			fights = append(fights, fight)
		})
	})

	// If we found no fights with the standard method, try an alternative table structure
	if len(fights) == 0 {
		// Try looking for different table structures
		doc.Find("table.wikitable").Each(func(i int, tableBody *goquery.Selection) {
			// Process this table if it looks like a fight card
			if tableBody.Find("tr th:contains('Weight class')").Length() > 0 {
				tableBody.Find("tr").Each(func(j int, row *goquery.Selection) {
					// Skip header row
					if j == 0 {
						return
					}

					cells := row.Find("td")
					if cells.Length() < 5 { // Minimum columns needed
						return
					}

					// Extract data from cells
					weightClass := strings.TrimSpace(cells.Eq(0).Text())
					fighter1Name := strings.TrimSpace(cells.Eq(1).Text())
					resultText := strings.TrimSpace(cells.Eq(2).Text())
					fighter2Name := strings.TrimSpace(cells.Eq(3).Text())
					method := ""

					if cells.Length() > 4 {
						method = strings.TrimSpace(cells.Eq(4).Text())
					}

					// Skip if essential data is missing
					if weightClass == "" || fighter1Name == "" || fighter2Name == "" {
						return
					}

					// Clean fighter names
					re := regexp.MustCompile(`\[\d+\]`)
					fighter1Name = strings.TrimSpace(re.ReplaceAllString(fighter1Name, ""))
					fighter2Name = strings.TrimSpace(re.ReplaceAllString(fighter2Name, ""))

					// Determine winner
					var fighter1Result, fighter2Result string
					if strings.Contains(resultText, "def.") {
						fighter1Result = "Win"
						fighter2Result = "Loss"
					}

					// Add to fights list
					fight := scrapers.UFCScrapedFight{
						Fighter1Name:     fighter1Name,
						Fighter2Name:     fighter2Name,
						Fighter1LastName: extractLastName(fighter1Name),
						Fighter2LastName: extractLastName(fighter2Name),
						Fighter1Result:   fighter1Result,
						Fighter2Result:   fighter2Result,
						WeightClass:      weightClass,
						IsMainEvent:      j == 1, // First non-header row is main event
						IsTitleFight:     strings.Contains(strings.ToLower(weightClass), "title"),
						Method:           method,
					}

					fights = append(fights, fight)
				})
			}
		})
	}

	// Add more logging to help diagnose issues
	if len(fights) == 0 {
		fmt.Println("Warning: No fights extracted from Wikipedia page")
		// Print table structure for debugging
		doc.Find("table").Each(func(i int, table *goquery.Selection) {
			className, _ := table.Attr("class")
			fmt.Printf("Found table with class: %s\n", className)
		})
	}

	return fights
}

// Helper function to extract last name
func extractLastName(fullName string) string {
	parts := strings.Fields(fullName)
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return fullName
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
		"á": "a",
		"à": "a",
		"â": "a",
		"ä": "a",
		"ã": "a",
		"å": "a",
		"ą": "a",
		"ă": "a",
		"ā": "a",

		"é": "e",
		"è": "e",
		"ê": "e",
		"ë": "e",
		"ę": "e",
		"ė": "e",
		"ě": "e",
		"ē": "e",

		"í": "i",
		"ì": "i",
		"î": "i",
		"ï": "i",
		"ı": "i", // Turkish dotless i
		"ī": "i",

		"ó": "o",
		"ò": "o",
		"ô": "o",
		"ö": "o", // This is in Özkılıç
		"õ": "o",
		"ø": "o",
		"ő": "o",
		"ō": "o",
		"ơ": "o",

		"ú": "u",
		"ù": "u",
		"û": "u",
		"ü": "u",
		"ū": "u",
		"ů": "u",
		"ű": "u",
		"ų": "u",
		"ư": "u",

		"ý": "y",
		"ÿ": "y",
		"ŷ": "y",

		"ç": "c",
		"č": "c",
		"ć": "c",
		"ĉ": "c",

		"ñ": "n",
		"ń": "n",
		"ň": "n",
		"ņ": "n",

		"ş": "s",
		"ś": "s",
		"š": "s",
		"ș": "s",
		"ŝ": "s",

		"ž": "z",
		"ź": "z",
		"ż": "z",

		"ł": "l",
		"ĺ": "l",
		"ļ": "l",
		"ľ": "l",

		"ť": "t",
		"ț": "t",
		"ţ": "t",

		"ď": "d",
		"đ": "d",

		"ř": "r",
		"ŕ": "r",
		"ŗ": "r",

		"ğ": "g", // This is in Lim Hyun-gyu
		"ĝ": "g",
		"ģ": "g",
		"ġ": "g",

		"ĵ": "j",
		"ķ": "k",
		"ĥ": "h",
		"ħ": "h",

		// Uppercase
		"Á": "A",
		"À": "A",
		"Â": "A",
		"Ä": "A",
		"Ã": "A",
		"Å": "A",
		"Ą": "A",
		"Ă": "A",
		"Ā": "A",

		"É": "E",
		"È": "E",
		"Ê": "E",
		"Ë": "E",
		"Ę": "E",
		"Ė": "E",
		"Ě": "E",
		"Ē": "E",

		"Í": "I",
		"Ì": "I",
		"Î": "I",
		"Ï": "I",
		"İ": "I", // Turkish dotted I
		"Ī": "I",

		"Ó": "O",
		"Ò": "O",
		"Ô": "O",
		"Ö": "O",
		"Õ": "O",
		"Ø": "O",
		"Ő": "O",
		"Ō": "O",
		"Ơ": "O",

		"Ú": "U",
		"Ù": "U",
		"Û": "U",
		"Ü": "U",
		"Ū": "U",
		"Ů": "U",
		"Ű": "U",
		"Ų": "U",
		"Ư": "U",

		"Ý": "Y",
		"Ÿ": "Y",
		"Ŷ": "Y",

		"Ç": "C",
		"Č": "C",
		"Ć": "C",
		"Ĉ": "C",

		"Ñ": "N",
		"Ń": "N",
		"Ň": "N",
		"Ņ": "N",

		"Ş": "S",
		"Ś": "S",
		"Š": "S",
		"Ș": "S",
		"Ŝ": "S",

		"Ž": "Z",
		"Ź": "Z",
		"Ż": "Z",

		"Ł": "L",
		"Ĺ": "L",
		"Ļ": "L",
		"Ľ": "L",

		"Ť": "T",
		"Ț": "T",
		"Ţ": "T",

		"Ď": "D",
		"Đ": "D",

		"Ř": "R",
		"Ŕ": "R",
		"Ŗ": "R",

		"Ğ": "G",
		"Ĝ": "G",
		"Ģ": "G",
		"Ġ": "G",

		"Ĵ": "J",
		"Ķ": "K",
		"Ĥ": "H",
		"Ħ": "H",
	}

	// Apply all replacements
	for accented, nonAccented := range replacements {
		name = strings.ReplaceAll(name, accented, nonAccented)
	}

	return name
}

func mapFighterName(name string) string {
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

	return name
}

// Enhanced tryEarlyUFCEvent function to handle more table structures
func tryEarlyUFCEvent(event struct {
	ID        string
	Name      string
	EventDate time.Time
	WikiURL   string
}) ([]scrapers.UFCScrapedFight, bool) {
	// Check if this is an early UFC event (UFC 1-30 or named events like UFC Brazil)
	isEarlyEvent := false
	eventNumber := extractUFCEventNumber(event.Name)

	// Check if it's a numbered event from 1-30
	if eventNumber != "" {
		num, err := strconv.Atoi(eventNumber)
		if err == nil && num <= 30 {
			isEarlyEvent = true
		}
	}

	// Also check for special named early events
	if strings.Contains(event.Name, "UFC Brazil") ||
		strings.Contains(event.Name, "Ultimate Japan") ||
		strings.Contains(event.Name, "Ultimate Ultimate") {
		isEarlyEvent = true
	}

	if !isEarlyEvent {
		return nil, false
	}

	log.Printf("Handling early UFC event: %s", event.Name)

	// For early events, we need to look for a different table structure
	resp, err := http.Get(event.WikiURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, false
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	resp.Body.Close()

	if err != nil {
		return nil, false
	}

	var fights []scrapers.UFCScrapedFight

	// Process all table.toccolours tables
	doc.Find("table.toccolours").Each(func(i int, table *goquery.Selection) {
		var currentSection string
		// Remove unused isMainCard variable

		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			// Check if this is a section header
			headerCells := row.Find("th[colspan]")
			if headerCells.Length() > 0 {
				headerText := strings.TrimSpace(headerCells.First().Text())
				currentSection = headerText

				// Remove the isMainCard assignment since it's not used
				return // Skip header row
			}

			// Skip column header rows
			if row.Find("th").Length() > 0 {
				return
			}

			// Process fight data rows
			cells := row.Find("td")
			if cells.Length() < 4 {
				return // Not enough cells for a fight
			}

			// Get the weight class from the first cell
			weightClass := strings.TrimSpace(cells.Eq(0).Text())
			if weightClass == "" {
				return // Skip rows without weight class
			}

			// Extract fighter names (cells 1 & 3)
			var fighter1Name, fighter2Name string

			// Try with links first, then fall back to text
			fighter1Cell := cells.Eq(1)
			fighter1Link := fighter1Cell.Find("a").First()
			if fighter1Link.Length() > 0 {
				fighter1Name = strings.TrimSpace(fighter1Link.Text())
			} else {
				fighter1Name = strings.TrimSpace(fighter1Cell.Text())
			}

			// Check for champion indicator and remove it
			if strings.Contains(fighter1Name, "(c)") {
				fighter1Name = strings.Replace(fighter1Name, "(c)", "", -1)
				fighter1Name = strings.TrimSpace(fighter1Name)
			}

			fighter2Cell := cells.Eq(3)
			fighter2Link := fighter2Cell.Find("a").First()
			if fighter2Link.Length() > 0 {
				fighter2Name = strings.TrimSpace(fighter2Link.Text())
			} else {
				fighter2Name = strings.TrimSpace(fighter2Cell.Text())
			}

			// Skip if either fighter name is missing
			if fighter1Name == "" || fighter2Name == "" {
				return
			}

			// Clean fighter names - remove footnotes
			re := regexp.MustCompile(`\[\w+\]`)
			fighter1Name = strings.TrimSpace(re.ReplaceAllString(fighter1Name, ""))
			fighter2Name = strings.TrimSpace(re.ReplaceAllString(fighter2Name, ""))

			// Get result (cell 2)
			var fighter1Result, fighter2Result string
			resultText := strings.TrimSpace(cells.Eq(2).Text())
			if strings.Contains(resultText, "def.") {
				fighter1Result = "Win"
				fighter2Result = "Loss"
			} else if strings.Contains(resultText, "draw") {
				fighter1Result = "Draw"
				fighter2Result = "Draw"
			}

			// Get method, round, time
			var method, round, time string

			// Method is usually in cell 4
			if cells.Length() > 4 {
				method = strings.TrimSpace(cells.Eq(4).Text())
			}

			// Round and time may be in cells 5 & 6, but some early events just have time
			if cells.Length() > 6 {
				round = strings.TrimSpace(cells.Eq(5).Text())
				time = strings.TrimSpace(cells.Eq(6).Text())
			} else if cells.Length() > 5 {
				// Check if cell 5 has time or round
				cell5Text := strings.TrimSpace(cells.Eq(5).Text())
				if strings.Contains(cell5Text, ":") {
					// Likely a time
					time = cell5Text
				} else {
					// Likely a round
					round = cell5Text
				}
			}

			// For UFC 17 style tables where there's no round column but time in cell 6
			if round == "" && cells.Length() > 6 {
				time = strings.TrimSpace(cells.Eq(6).Text())
			}

			// Check if this is a title fight
			isTitleFight := strings.Contains(strings.ToLower(currentSection), "championship") ||
				strings.Contains(strings.ToLower(weightClass), "title")

			// Determine if this is the main event
			// For early UFC events, championship fights or the first fight in the first section are main events
			isMainEvent := (j == 2 && i == 0) || // First fight in first table
				strings.Contains(strings.ToLower(currentSection), "championship") || // Championship section
				(currentSection != "" && strings.Contains(strings.ToLower(currentSection), "main")) // Main event section

			// Create the fight
			fight := scrapers.UFCScrapedFight{
				Fighter1Name:     fighter1Name,
				Fighter2Name:     fighter2Name,
				Fighter1LastName: extractLastName(fighter1Name),
				Fighter2LastName: extractLastName(fighter2Name),
				Fighter1Result:   fighter1Result,
				Fighter2Result:   fighter2Result,
				WeightClass:      weightClass,
				IsMainEvent:      isMainEvent,
				IsTitleFight:     isTitleFight,
				Method:           method,
				Round:            round,
				Time:             time,
			}

			fights = append(fights, fight)
		})
	})

	// If we found fights, return them
	if len(fights) > 0 {
		return fights, true
	}

	// If no fights found, try more aggressive methods (older events with different table formats)
	// Search for any tables with fight data
	doc.Find("table").Each(func(i int, table *goquery.Selection) {
		table.Find("tr").Each(func(j int, row *goquery.Selection) {
			// Skip header rows
			if row.Find("th").Length() > 0 {
				return
			}

			cells := row.Find("td")
			if cells.Length() < 3 {
				return // Need at least fighter1, result, fighter2
			}

			// Try to detect if this is a fight row
			var isFightRow bool
			var fighter1Name, fighter2Name, weightClass, method string

			// Look for text like "def." which indicates a fight result
			row.Find("td").Each(func(k int, cell *goquery.Selection) {
				cellText := strings.TrimSpace(cell.Text())
				if strings.Contains(cellText, "def.") ||
					strings.Contains(strings.ToLower(cellText), "defeat") ||
					strings.Contains(strings.ToLower(cellText), "submission") ||
					strings.Contains(strings.ToLower(cellText), "tko") ||
					strings.Contains(strings.ToLower(cellText), "knockout") {
					isFightRow = true
				}
			})

			if !isFightRow {
				return
			}

			// Try different patterns to extract fighter names and other info
			// Pattern 1: Fighter1 | def. | Fighter2
			if cells.Length() >= 3 {
				fighter1Name = strings.TrimSpace(cells.Eq(0).Text())
				// Use resultText instead of just declaring it
				var resultType string
				resultType = strings.TrimSpace(cells.Eq(1).Text())
				fighter2Name = strings.TrimSpace(cells.Eq(2).Text())

				// Check for links
				fighter1Link := cells.Eq(0).Find("a").First()
				if fighter1Link.Length() > 0 {
					fighter1Name = strings.TrimSpace(fighter1Link.Text())
				}

				fighter2Link := cells.Eq(2).Find("a").First()
				if fighter2Link.Length() > 0 {
					fighter2Name = strings.TrimSpace(fighter2Link.Text())
				}

				// If we have more cells, try to extract method
				if cells.Length() > 3 {
					method = strings.TrimSpace(cells.Eq(3).Text())
				}

				// Default weight class for very old events
				weightClass = "Heavyweight" // Early UFC was often open weight

				// Create the fight if we have enough data
				if fighter1Name != "" && fighter2Name != "" {
					// Clean fighter names
					re := regexp.MustCompile(`\[\w+\]`)
					fighter1Name = strings.TrimSpace(re.ReplaceAllString(fighter1Name, ""))
					fighter2Name = strings.TrimSpace(re.ReplaceAllString(fighter2Name, ""))

					// Determine result based on resultType if possible
					var fighter1Result, fighter2Result string
					if strings.Contains(resultType, "def.") {
						fighter1Result = "Win"
						fighter2Result = "Loss"
					} else {
						fighter1Result = "Win"  // Default
						fighter2Result = "Loss" // Default
					}

					fight := scrapers.UFCScrapedFight{
						Fighter1Name:     fighter1Name,
						Fighter2Name:     fighter2Name,
						Fighter1LastName: extractLastName(fighter1Name),
						Fighter2LastName: extractLastName(fighter2Name),
						Fighter1Result:   fighter1Result,
						Fighter2Result:   fighter2Result,
						WeightClass:      weightClass,
						IsMainEvent:      j == 0, // First fight is often main event
						Method:           method,
					}

					fights = append(fights, fight)
				}
			}
		})
	})

	// If we found any fights, consider it successful
	if len(fights) > 0 {
		return fights, true
	}

	return nil, false
}
