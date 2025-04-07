package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
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

	// Define manual fighters that couldn't be scraped
	manualFighters := []map[string]string{
		{
			"ufc_id":          "daniel-sarafian",
			"name":            "Daniel Sarafian",
			"nickname":        "",
			"weight_class":    "Light Heavyweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "São Paulo, Brazil",
			"height":          "69.00",
			"weight":          "205.00",
			"age":             "21 August 1982",
			"nationality":     "Brazil",
			"wiki_url":        "https://en.wikipedia.org/wiki/Daniel_Sarafian",
			"ufc_url":         "",
			"reach":           "70.00",
			"wins":            "11",
			"losses":          "6",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "1",
			"sub_wins":        "7",
			"dec_wins":        "3",
			"loss_by_ko":      "2",
			"loss_by_sub":     "1",
			"loss_by_dec":     "3",
		},
		{
			"ufc_id":          "godofredo-pepey",
			"name":            "Godofredo Pepey",
			"nickname":        "",
			"weight_class":    "Featherweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Fortaleza, Ceará, Brazil",
			"height":          "71.00",
			"weight":          "145.00",
			"age":             "July 2, 1987",
			"nationality":     "Brazil",
			"wiki_url":        "https://en.wikipedia.org/wiki/Godofredo_Pepey",
			"ufc_url":         "",
			"reach":           "74.8",
			"wins":            "13",
			"losses":          "7",
			"draws":           "0",
			"no_contests":     "1",
			"ko_wins":         "4",
			"sub_wins":        "8",
			"dec_wins":        "1",
			"loss_by_ko":      "3",
			"loss_by_sub":     "1",
			"loss_by_dec":     "3",
		},
		{
			"ufc_id":          "goran-reljic",
			"name":            "Goran Reljic",
			"nickname":        "Ghost",
			"weight_class":    "Light Heavyweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Zadar, Croatia",
			"height":          "75.20",
			"weight":          "205.00",
			"age":             "20 March 1984",
			"nationality":     "Croatia",
			"wiki_url":        "https://en.wikipedia.org/wiki/Goran_Relji%C4%87",
			"ufc_url":         "",
			"reach":           "81.10",
			"wins":            "21",
			"losses":          "11",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "7",
			"sub_wins":        "7",
			"dec_wins":        "7",
			"loss_by_ko":      "4",
			"loss_by_sub":     "0",
			"loss_by_dec":     "7",
		},
		{
			"ufc_id":          "jason-godsey",
			"name":            "Jason Godsey",
			"nickname":        "The Indianaderthal",
			"weight_class":    "Heavyweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Indianapolis, Indiana",
			"height":          "74.01",
			"weight":          "251.00",
			"age":             "January 1, 1964",
			"nationality":     "United States",
			"wiki_url":        "",
			"ufc_url":         "",
			"reach":           "",
			"wins":            "16",
			"losses":          "16",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "0",
			"sub_wins":        "16",
			"dec_wins":        "0",
			"loss_by_ko":      "5",
			"loss_by_sub":     "9",
			"loss_by_dec":     "2",
		},
		{
			"ufc_id":          "joão-zeferino",
			"name":            "João Zeferino",
			"nickname":        "The Brazilian Samurai",
			"weight_class":    "Welterweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Middletown, New York",
			"height":          "71.00",
			"weight":          "170.00",
			"age":             "1/15/1986",
			"nationality":     "Brazil",
			"wiki_url":        "",
			"ufc_url":         "https://www.ufc.com/athlete/joao-zeferino",
			"reach":           "69.00",
			"wins":            "26",
			"losses":          "11",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "4",
			"sub_wins":        "17",
			"dec_wins":        "5",
			"loss_by_ko":      "4",
			"loss_by_sub":     "0",
			"loss_by_dec":     "7",
		},
		{
			"ufc_id":          "jonathan-brookins",
			"name":            "Jonathan Brookins",
			"nickname":        "",
			"weight_class":    "Featherweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Orlando, Florida",
			"height":          "72.00",
			"weight":          "145.00",
			"age":             "",
			"nationality":     "United States",
			"wiki_url":        "https://en.wikipedia.org/wiki/Jonathan_Brookins",
			"ufc_url":         "https://www.ufc.com/athlete/jonathan-brookins",
			"reach":           "74",
			"wins":            "16",
			"losses":          "10",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "3",
			"sub_wins":        "9",
			"dec_wins":        "4",
			"loss_by_ko":      "1",
			"loss_by_sub":     "2",
			"loss_by_dec":     "7",
		},
		{
			"ufc_id":          "kazuki-tokudome",
			"name":            "Kazuki Tokudome",
			"nickname":        "",
			"weight_class":    "Lightweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Tokyo, Japan",
			"height":          "71.00",
			"weight":          "155.00",
			"age":             "",
			"nationality":     "Japan",
			"wiki_url":        "https://en.wikipedia.org/wiki/Kazuki_Tokudome",
			"ufc_url":         "https://www.ufc.com/athlete/kazuki-tokudome",
			"reach":           "73.00",
			"wins":            "20",
			"losses":          "12",
			"draws":           "1",
			"no_contests":     "0",
			"ko_wins":         "10",
			"sub_wins":        "3",
			"dec_wins":        "7",
			"loss_by_ko":      "6",
			"loss_by_sub":     "3",
			"loss_by_dec":     "3",
		},
		{
			"ufc_id":          "renée-forte",
			"name":            "Renée Forte",
			"nickname":        "",
			"weight_class":    "Lightweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Fortaleza, Ceará, Brazil",
			"height":          "67.00",
			"weight":          "155.00",
			"age":             "March 27, 1987",
			"nationality":     "Brazil",
			"wiki_url":        "https://en.wikipedia.org/wiki/Ren%C3%A9e_Forte",
			"ufc_url":         "",
			"reach":           "71.00",
			"wins":            "8",
			"losses":          "4",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "2",
			"sub_wins":        "2",
			"dec_wins":        "4",
			"loss_by_ko":      "2",
			"loss_by_sub":     "1",
			"loss_by_dec":     "1",
		},
		{
			"ufc_id":          "yoshiyuki-yoshida",
			"name":            "Yoshiyuki Yoshida",
			"nickname":        "Zenko",
			"weight_class":    "Welterweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Albuquerque, New Mexico",
			"height":          "71.00",
			"weight":          "167.00",
			"age":             "May 10, 1974",
			"nationality":     "Japan",
			"wiki_url":        "https://en.wikipedia.org/wiki/Yoshiyuki_Yoshida",
			"ufc_url":         "",
			"reach":           "70.00",
			"wins":            "2",
			"losses":          "3",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "0",
			"sub_wins":        "2",
			"dec_wins":        "0",
			"loss_by_ko":      "2",
			"loss_by_sub":     "0",
			"loss_by_dec":     "1",
		},
		{
			"ufc_id":          "zelim-imadaev",
			"name":            "Zelim Imadaev",
			"nickname":        "",
			"weight_class":    "Welterweight",
			"rank":            "Unranked",
			"status":          "Retired",
			"fighting_out_of": "Chechnya, Russia",
			"height":          "72.00",
			"weight":          "170.00",
			"age":             "January 25, 1995",
			"nationality":     "Russia",
			"wiki_url":        "https://en.wikipedia.org/wiki/Zelim_Imadaev",
			"ufc_url":         "",
			"reach":           "76.00",
			"wins":            "8",
			"losses":          "3",
			"draws":           "0",
			"no_contests":     "0",
			"ko_wins":         "8",
			"sub_wins":        "0",
			"dec_wins":        "0",
			"loss_by_ko":      "1",
			"loss_by_sub":     "1",
			"loss_by_dec":     "1",
		},
	}

	// Loop through manual fighters and insert each one
	log.Println("Inserting manual fighters that couldn't be scraped...")
	for _, fighter := range manualFighters {
		// Parse record
		wins, err := strconv.Atoi(fighter["wins"])
		if err != nil {
			wins = 0
		}
		losses, err := strconv.Atoi(fighter["losses"])
		if err != nil {
			losses = 0
		}
		draws, err := strconv.Atoi(fighter["draws"])
		if err != nil {
			draws = 0
		}
		noContests, err := strconv.Atoi(fighter["no_contests"])
		if err != nil {
			noContests = 0
		}

		// Parse win types
		koWins, err := strconv.Atoi(fighter["ko_wins"])
		if err != nil {
			koWins = 0
		}
		subWins, err := strconv.Atoi(fighter["sub_wins"])
		if err != nil {
			subWins = 0
		}
		decWins, err := strconv.Atoi(fighter["dec_wins"])
		if err != nil {
			decWins = 0
		}

		// Parse loss types
		lossByKo, err := strconv.Atoi(fighter["loss_by_ko"])
		if err != nil {
			lossByKo = 0
		}
		lossBySub, err := strconv.Atoi(fighter["loss_by_sub"])
		if err != nil {
			lossBySub = 0
		}
		lossByDec, err := strconv.Atoi(fighter["loss_by_dec"])
		if err != nil {
			lossByDec = 0
		}

		// Parse age
		var age int
		var birthDateErr error
		if fighter["age"] != "" {
			// Try to parse different date formats
			dateFormats := []string{
				"January 2, 2006", // "January 1, 1982"
				"1/2/2006",        // "1/15/1986"
				"2006-01-02",      // "1982-08-21"
				"2 January 2006",  // "10 May 1974"
				"Jan 2, 2006",     // "Aug 21, 1982"
			}

			for _, format := range dateFormats {
				var birthDateStr string
				var parseErr error

				// Try to parse the date
				birthDate, parseErr := time.Parse(format, fighter["age"])
				if parseErr == nil {
					// Format the date to the expected "YYYY-MM-DD" format
					birthDateStr = birthDate.Format("2006-01-02")

					// Calculate age using the existing function
					age, birthDateErr = calculateAge(birthDateStr)
					if birthDateErr == nil {
						break
					}
				}
			}

			// Log any errors in parsing the age
			if birthDateErr != nil {
				log.Printf("Warning: Could not parse age for %s: %v", fighter["name"], birthDateErr)
			}
		}

		// Insert or update fighter
		_, err = db.Exec(`
        INSERT INTO fighters (
            ufc_id, name, nickname, weight_class, status, rank, 
            fighting_out_of, height, weight, age, nationality, 
            wiki_url, ufc_url, reach,
            wins, losses, draws, no_contests,
            ko_wins, sub_wins, dec_wins,
            loss_by_ko, loss_by_sub, loss_by_dec,
            created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, 
            $7, $8, $9, $10, $11, 
            $12, $13, $14,
            $15, $16, $17, $18,
            $19, $20, $21,
            $22, $23, $24,
            $25, $25
        )
        ON CONFLICT (ufc_id) DO UPDATE SET
            name = EXCLUDED.name,
            nickname = EXCLUDED.nickname,
            weight_class = EXCLUDED.weight_class,
            status = EXCLUDED.status,
            rank = EXCLUDED.rank,
            fighting_out_of = EXCLUDED.fighting_out_of,
            height = EXCLUDED.height,
            weight = EXCLUDED.weight,
            age = EXCLUDED.age,
            nationality = EXCLUDED.nationality,
            wiki_url = EXCLUDED.wiki_url,
            ufc_url = EXCLUDED.ufc_url,
            reach = EXCLUDED.reach,
            wins = EXCLUDED.wins,
            losses = EXCLUDED.losses,
            draws = EXCLUDED.draws,
            no_contests = EXCLUDED.no_contests,
            ko_wins = EXCLUDED.ko_wins,
            sub_wins = EXCLUDED.sub_wins,
            dec_wins = EXCLUDED.dec_wins,
            loss_by_ko = EXCLUDED.loss_by_ko,
            loss_by_sub = EXCLUDED.loss_by_sub,
            loss_by_dec = EXCLUDED.loss_by_dec,
            updated_at = EXCLUDED.updated_at`,
			fighter["ufc_id"],
			fighter["name"],
			fighter["nickname"],
			fighter["weight_class"],
			fighter["status"],
			fighter["rank"],
			fighter["fighting_out_of"],
			fighter["height"], // Already a string from the map
			fighter["weight"], // Already a string from the map
			age,
			fighter["nationality"],
			fighter["wiki_url"],
			fighter["ufc_url"],
			fighter["reach"], // Already a string from the map
			wins,
			losses,
			draws,
			noContests,
			koWins,
			subWins,
			decWins,
			lossByKo,
			lossBySub,
			lossByDec,
			time.Now(),
		)
		if err != nil {
			log.Printf("Error inserting fighter %s: %v", fighter["name"], err)
		} else {
			log.Printf("Successfully inserted fighter: %s", fighter["name"])
		}
	}

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

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer dbCancel()

	// Track success counts
	insertCount := 0
	updateCount := 0
	errorCount := 0
	rankingsInserted := 0

	// Process each fighter in its own transaction
	for _, fighter := range fighters {
		// Start a new transaction for each fighter
		tx, err := db.BeginTx(dbCtx, nil)
		if err != nil {
			log.Printf("Failed to begin transaction for fighter %s: %v", fighter.Name, err)
			errorCount++
			continue
		}

		// Ensure the transaction is either committed or rolled back
		var committed bool
		defer func() {
			if tx != nil && !committed {
				tx.Rollback()
			}
		}()

		// Parse record to get wins, losses, draws
		wins, losses, draws := 0, 0, 0

		// Parse the record string (typically in format "W-L-D")
		if fighter.Record != "" {
			parts := strings.Split(fighter.Record, "-")
			if len(parts) >= 3 {
				// Using strconv to parse the numbers
				wins, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
				losses, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
				draws, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
			}
		}

		// First check if fighter exists
		var existingID string

		err = tx.QueryRowContext(dbCtx,
			"SELECT id FROM fighters WHERE ufc_id = $1", fighter.UFCID).Scan(&existingID)

		if err != nil && err != sql.ErrNoRows {
			log.Printf("Error checking for existing fighter %s: %v", fighter.Name, err)
			tx.Rollback()
			tx = nil
			errorCount++
			continue
		}

		now := time.Now()

		// If fighter exists, update it
		if err == nil {
			// This is an update
			_, err = tx.ExecContext(dbCtx, `
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
				WHERE id = $19`,
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
				log.Printf("Failed to update fighter %s: %v", fighter.Name, err)
				tx.Rollback()
				tx = nil
				errorCount++
				continue
			}

			updateCount++
		} else {
			// Insert new fighter
			err = tx.QueryRowContext(dbCtx, `
				INSERT INTO fighters (
					ufc_id, name, nickname, weight_class, status, rank, wins, losses, draws, ufc_url,
					age, height, weight, reach, ko_wins, sub_wins, dec_wins, nationality,
					created_at, updated_at
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $19
				) RETURNING id`,
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
				log.Printf("Failed to insert fighter %s: %v", fighter.Name, err)
				tx.Rollback()
				tx = nil
				errorCount++
				continue
			}

			insertCount++
		}

		// Process rankings if they exist
		if len(fighter.Rankings) > 0 {
			// First delete any existing rankings for this fighter
			_, err = tx.ExecContext(dbCtx,
				"DELETE FROM fighter_rankings WHERE fighter_id = $1",
				existingID)

			if err != nil {
				log.Printf("Failed to delete existing rankings for fighter %s: %v", fighter.Name, err)
				tx.Rollback()
				tx = nil
				errorCount++
				continue
			}

			// Insert each ranking
			rankingSuccess := true
			fighterRankingsCount := 0

			for _, ranking := range fighter.Rankings {
				_, err = tx.ExecContext(dbCtx, `
					INSERT INTO fighter_rankings (
						fighter_id, weight_class, rank, created_at, updated_at
					) VALUES ($1, $2, $3, $4, $4)`,
					existingID,
					ranking.WeightClass,
					ranking.Rank,
					now,
				)

				if err != nil {
					rankingSuccess = false
					break
				}

				fighterRankingsCount++
			}

			if !rankingSuccess {
				tx.Rollback()
				tx = nil
				errorCount++
				continue
			}

			rankingsInserted += fighterRankingsCount
		}

		if err = tx.Commit(); err != nil {
			tx.Rollback()
			tx = nil
			errorCount++
			continue
		}

		committed = true
		tx = nil
	}

	log.Printf("Scraping completed! Saved %d new fighters, updated %d existing fighters with %d total rankings. Errors: %d",
		insertCount, updateCount, rankingsInserted, errorCount)
}

// calculateAge calculates a person's age in years given their birth date
func calculateAge(birthDateStr string) (int, error) {
	// Parse the birthdate string (assuming format "YYYY-MM-DD")
	birthDate, err := time.Parse("2006-01-02", birthDateStr)
	if err != nil {
		return 0, fmt.Errorf("error parsing birth date: %w", err)
	}

	// Get current date
	now := time.Now()

	// Calculate age
	age := now.Year() - birthDate.Year()

	// Adjust age if birthday hasn't occurred yet this year
	if now.Month() < birthDate.Month() ||
		(now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}

	return age, nil
}
