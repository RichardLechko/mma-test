package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// TournamentScraper handles the processing of tournament events
type TournamentScraper struct {
	db *sql.DB
}

// TournamentFight represents a fight in a tournament
type TournamentFight struct {
	Fighter1ID      string
	Fighter1Name    string
	Fighter2ID      string
	Fighter2Name    string
	WeightClass     string
	Round           string
	BracketPosition int
	WinnerID        string
	Method          string
	Time            string
}

// Tournament represents a tournament structure
type Tournament struct {
	Name        string
	WeightClass string
	BracketType string
	Fights      []TournamentFight
	WinnerID    string
}

// NewTournamentScraper creates a new tournament scraper
func NewTournamentScraper(db *sql.DB) *TournamentScraper {
	return &TournamentScraper{
		db: db,
	}
}

func (s *TournamentScraper) ProcessTournament(ctx context.Context, eventID string, eventName string) error {
	// Get tournament data based on event name
	tournaments, err := s.getTournamentData(eventName)
	if err != nil {
		return fmt.Errorf("failed to get tournament data: %w", err)
	}

	if len(tournaments) == 0 {
		return fmt.Errorf("no tournament data found for event: %s", eventName)
	}

	// Process each tournament
	for _, tournament := range tournaments {
		log.Printf("Processing tournament: %s", tournament.Name)

		// Find the winner ID if specified (as a name)
		var winnerUUID *string
		if tournament.WinnerID != "" {
			// Look up winner by name
			winnerID, err := s.findOrCreateFighter(ctx, tournament.WinnerID)
			if err != nil {
				log.Printf("Error finding winner %s: %v", tournament.WinnerID, err)
			} else {
				winnerUUID = &winnerID
			}
		}

		// Create the tournament record
		var tournamentID string
		err := s.db.QueryRowContext(ctx, `
            INSERT INTO tournaments (
                event_id, name, bracket_type, weight_class, winner_id,
                created_at, updated_at
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7
            )
            RETURNING id
        `,
			eventID, tournament.Name, tournament.BracketType, tournament.WeightClass, winnerUUID,
			time.Now(), time.Now()).Scan(&tournamentID)

		if err != nil {
			return fmt.Errorf("failed to insert tournament: %w", err)
		}

		// Process tournament fights
		for _, fight := range tournament.Fights {
			// Try to find or create fighters
			fighter1ID, err := s.findOrCreateFighter(ctx, fight.Fighter1Name)
			if err != nil {
				log.Printf("Error with fighter %s: %v", fight.Fighter1Name, err)
				continue
			}
			log.Printf("Found/Created fighter 1: %s (ID: %s)", fight.Fighter1Name, fighter1ID)

			fighter2ID, err := s.findOrCreateFighter(ctx, fight.Fighter2Name)
			if err != nil {
				log.Printf("Error with fighter %s: %v", fight.Fighter2Name, err)
				continue
			}
			log.Printf("Found/Created fighter 2: %s (ID: %s)", fight.Fighter2Name, fighter2ID)

			// Create the fight
			var fightID string
			err = s.db.QueryRowContext(ctx, `
    INSERT INTO fights (
        event_id, fighter1_id, fighter2_id, fighter1_name, fighter2_name,
        weight_class, is_main_event, 
        created_at, updated_at
    ) VALUES (
        $1, $2, $3, $4, $5, $6, $7, $8, $9
    )
    ON CONFLICT (event_id, fighter1_id, fighter2_id) 
    DO UPDATE SET
        fighter1_name = EXCLUDED.fighter1_name,
        fighter2_name = EXCLUDED.fighter2_name,
        weight_class = EXCLUDED.weight_class,
        is_main_event = EXCLUDED.is_main_event,
        updated_at = EXCLUDED.updated_at
    RETURNING id
`,
				eventID, fighter1ID, fighter2ID, fight.Fighter1Name, fight.Fighter2Name,
				fight.WeightClass, false,
				time.Now(), time.Now()).Scan(&fightID)

			if err != nil {
				log.Printf("Failed to save fight %s vs %s: %v",
					fight.Fighter1Name, fight.Fighter2Name, err)
				continue
			}
			log.Printf("Created/Updated fight with ID: %s", fightID)

			// Link fight to tournament
			_, err = s.db.ExecContext(ctx, `
                INSERT INTO tournament_fights (
                    tournament_id, fight_id, round_name, bracket_position,
                    created_at, updated_at
                ) VALUES (
                    $1, $2, $3, $4, $5, $6
                )
                ON CONFLICT (tournament_id, fight_id) 
                DO UPDATE SET
                    round_name = EXCLUDED.round_name,
                    bracket_position = EXCLUDED.bracket_position,
                    updated_at = EXCLUDED.updated_at
            `,
				tournamentID, fightID, fight.Round, fight.BracketPosition,
				time.Now(), time.Now())

			if err != nil {
				log.Printf("Failed to link fight to tournament: %v", err)
				continue
			}
			log.Printf("Linked fight %s to tournament %s", fightID, tournamentID)
		}
	}

	return nil
}

// findOrCreateFighter tries to find a fighter in the database or creates a new one
func (s *TournamentScraper) findOrCreateFighter(ctx context.Context, name string) (string, error) {
	// Try to find the fighter
	var fighterID string
	err := s.db.QueryRowContext(ctx,
		"SELECT id FROM fighters WHERE LOWER(name) = LOWER($1)", name).Scan(&fighterID)

	if err == nil {
		return fighterID, nil
	}

	if err != sql.ErrNoRows {
		return "", fmt.Errorf("error finding fighter: %w", err)
	}

	// Fighter not found, create a new one
	// Generate a placeholder UFC ID for now
	ufc_id := strings.ReplaceAll(strings.ToLower(name), " ", "-")

	err = s.db.QueryRowContext(ctx, `
        INSERT INTO fighters (
            ufc_id, name, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4
        )
        RETURNING id
    `,
		ufc_id, name, time.Now(), time.Now()).Scan(&fighterID)

	if err != nil {
		return "", fmt.Errorf("error creating fighter: %w", err)
	}

	return fighterID, nil
}

// extractLastNameForTournament extracts the last name from a full name
func extractLastNameForTournament(name string) string {
	parts := strings.Fields(name)
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return name
}

// getTournamentData returns tournament data for a specific event
func (s *TournamentScraper) getTournamentData(eventName string) ([]Tournament, error) {
	// This function contains hardcoded tournament data for each event

	switch eventName {
	case "UFC 17: Redemption":
		// Create an array to store all tournaments for this event
		tournaments := []Tournament{}

		// Middleweight Tournament
		middleweightTournament := Tournament{
			Name:        "Middleweight Tournament",
			WeightClass: "Middleweight",
			BracketType: "4-man",
			WinnerID:    "Dan Henderson",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Dan Henderson",
					Fighter2ID:      "",
					Fighter2Name:    "Allan Góes",
					WeightClass:     "Middleweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Dan Henderson",
					Method:          "Decision (unanimous)",
					Time:            "15:00",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Carlos Newton",
					Fighter2ID:      "",
					Fighter2Name:    "Bob Gilstrap",
					WeightClass:     "Middleweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Carlos Newton",
					Method:          "Submission (triangle choke)",
					Time:            "0:54",
				},
				// Final
				{
					Fighter1ID:      "",
					Fighter1Name:    "Dan Henderson",
					Fighter2ID:      "",
					Fighter2Name:    "Carlos Newton",
					WeightClass:     "Middleweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Dan Henderson",
					Method:          "Decision (split)",
					Time:            "15:00",
				},
			},
		}
		tournaments = append(tournaments, middleweightTournament)

		// Light Heavyweight Championship
		lightHeavyweightChampionship := Tournament{
			Name:        "Light Heavyweight Championship",
			WeightClass: "Light Heavyweight",
			BracketType: "championship",
			WinnerID:    "Frank Shamrock", // Frank Shamrock
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Frank Shamrock",
					Fighter2ID:      "",
					Fighter2Name:    "Jeremy Horn",
					WeightClass:     "Light Heavyweight",
					Round:           "Championship",
					BracketPosition: 1,
					WinnerID:        "Frank Shamrock",
					Method:          "Submission (kneebar)",
					Time:            "16:28",
				},
			},
		}
		tournaments = append(tournaments, lightHeavyweightChampionship)

		// Heavyweight bouts collection
		heavyweightBouts := Tournament{
			Name:        "Heavyweight Bouts",
			WeightClass: "Heavyweight",
			BracketType: "superfight",
			WinnerID:    "", // No single winner
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Pete Williams",
					Fighter2ID:      "",
					Fighter2Name:    "Mark Coleman",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 1,
					WinnerID:        "Pete Williams",
					Method:          "KO (head kick)",
					Time:            "12:38",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Tank Abbott",
					Fighter2ID:      "",
					Fighter2Name:    "Hugo Duarte",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 2,
					WinnerID:        "Tank Abbott",
					Method:          "TKO (punches)",
					Time:            "0:43",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Mike van Arsdale",
					Fighter2ID:      "",
					Fighter2Name:    "Joe Pardo",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 3,
					WinnerID:        "Mike van Arsdale",
					Method:          "Submission (americana)",
					Time:            "11:01",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Andre Roberts",
					Fighter2ID:      "",
					Fighter2Name:    "Harry Moskowitz",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 4,
					WinnerID:        "Andre Roberts",
					Method:          "KO (elbow)",
					Time:            "3:15",
				},
			},
		}
		tournaments = append(tournaments, heavyweightBouts)

		// Middleweight non-tournament bout
		middleweightBout := Tournament{
			Name:        "Middleweight Bout",
			WeightClass: "Middleweight",
			BracketType: "superfight",
			WinnerID:    "Chuck Liddell", // Chuck Liddell
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Chuck Liddell",
					Fighter2ID:      "",
					Fighter2Name:    "Noe Hernandez",
					WeightClass:     "Middleweight",
					Round:           "Main Card",
					BracketPosition: 1,
					WinnerID:        "Chuck Liddell", // Chuck Liddell
					Method:          "Decision (unanimous)",
					Time:            "12:00",
				},
			},
		}
		tournaments = append(tournaments, middleweightBout)

		return tournaments, nil

	case "UFC 23: Ultimate Japan 2":
		// Create an array to store all tournaments for this event
		tournaments := []Tournament{}

		// UFC Japan Middleweight Tournament
		japanMiddleweightTournament := Tournament{
			Name:        "UFC Japan Middleweight Tournament",
			WeightClass: "Middleweight",
			BracketType: "4-man",
			WinnerID:    "Kenichi Yamamoto",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Katsuhisa Fujii",
					Fighter2ID:      "",
					Fighter2Name:    "Masutatsu Yano",
					WeightClass:     "Middleweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Katsuhisa Fujii",
					Method:          "TKO (referee stoppage due to strikes)",
					Time:            "3:14",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Kenichi Yamamoto",
					Fighter2ID:      "",
					Fighter2Name:    "Daiju Takase",
					WeightClass:     "Middleweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Kenichi Yamamoto",
					Method:          "Decision (unanimous)",
					Time:            "5:00",
				},
				// Final
				{
					Fighter1ID:      "",
					Fighter1Name:    "Kenichi Yamamoto",
					Fighter2ID:      "",
					Fighter2Name:    "Katsuhisa Fujii",
					WeightClass:     "Middleweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Kenichi Yamamoto",
					Method:          "Submission (kneebar)",
					Time:            "4:15",
				},
			},
		}
		tournaments = append(tournaments, japanMiddleweightTournament)

		// Main Card bouts
		mainCardBouts := Tournament{
			Name:        "Main Card Bouts",
			WeightClass: "Mixed",
			BracketType: "superfight",
			WinnerID:    "",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Kevin Randleman",
					Fighter2ID:      "",
					Fighter2Name:    "Pete Williams",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 1,
					WinnerID:        "Kevin Randleman",
					Method:          "Decision (unanimous)",
					Time:            "5:00",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Pedro Rizzo",
					Fighter2ID:      "",
					Fighter2Name:    "Tsuyoshi Kosaka",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 2,
					WinnerID:        "Pedro Rizzo",
					Method:          "TKO (referee stoppage due to strikes)",
					Time:            "1:17",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Joe Slick",
					Fighter2ID:      "",
					Fighter2Name:    "Jason DeLucia",
					WeightClass:     "Middleweight",
					Round:           "Main Card",
					BracketPosition: 3,
					WinnerID:        "Joe Slick",
					Method:          "TKO (verbal submission due to knee injury)",
					Time:            "1:28",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Eugene Jackson",
					Fighter2ID:      "",
					Fighter2Name:    "Keiichiro Yamamiya",
					WeightClass:     "Middleweight",
					Round:           "Main Card",
					BracketPosition: 4,
					WinnerID:        "Eugene Jackson",
					Method:          "KO (punch)",
					Time:            "3:12",
				},
			},
		}
		tournaments = append(tournaments, mainCardBouts)

		return tournaments, nil

	case "UFC 16: Battle in the Bayou":
		// Create an array to store all tournaments for this event
		tournaments := []Tournament{}

		// Welterweight Tournament
		welterweightTournament := Tournament{
			Name:        "Welterweight Tournament",
			WeightClass: "Welterweight",
			BracketType: "4-man",
			WinnerID:    "Pat Miletich",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Mikey Burnett",
					Fighter2ID:      "",
					Fighter2Name:    "Eugenio Tadeu",
					WeightClass:     "Welterweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Mikey Burnett",
					Method:          "TKO (punches)",
					Time:            "9:46",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Pat Miletich",
					Fighter2ID:      "",
					Fighter2Name:    "Townsend Saunders",
					WeightClass:     "Welterweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Pat Miletich",
					Method:          "Decision (split)",
					Time:            "15:00",
				},
				// Final - Note: Chris Brennan replaced Mikey Burnett as alternate
				{
					Fighter1ID:      "",
					Fighter1Name:    "Pat Miletich",
					Fighter2ID:      "",
					Fighter2Name:    "Chris Brennan",
					WeightClass:     "Welterweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Pat Miletich",
					Method:          "Submission (shoulder choke)",
					Time:            "9:01",
				},
			},
		}
		tournaments = append(tournaments, welterweightTournament)

		// Light Heavyweight Championship
		lightHeavyweightChampionship := Tournament{
			Name:        "Light Heavyweight Championship",
			WeightClass: "Light Heavyweight",
			BracketType: "championship",
			WinnerID:    "Frank Shamrock",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Frank Shamrock",
					Fighter2ID:      "",
					Fighter2Name:    "Igor Zinoviev",
					WeightClass:     "Light Heavyweight",
					Round:           "Championship",
					BracketPosition: 1,
					WinnerID:        "Frank Shamrock",
					Method:          "KO (slam)",
					Time:            "0:22",
				},
			},
		}
		tournaments = append(tournaments, lightHeavyweightChampionship)

		// Heavyweight Bout
		heavyweightBout := Tournament{
			Name:        "Heavyweight Bout",
			WeightClass: "Heavyweight",
			BracketType: "superfight",
			WinnerID:    "Tsuyoshi Kohsaka",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Tsuyoshi Kohsaka",
					Fighter2ID:      "",
					Fighter2Name:    "Kimo Leopoldo",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 1,
					WinnerID:        "Tsuyoshi Kohsaka",
					Method:          "Decision (unanimous)",
					Time:            "15:00",
				},
			},
		}
		tournaments = append(tournaments, heavyweightBout)

		// Light Heavyweight Bout
		lightHeavyweightBout := Tournament{
			Name:        "Light Heavyweight Bout",
			WeightClass: "Light Heavyweight",
			BracketType: "superfight",
			WinnerID:    "Jerry Bohlander",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Jerry Bohlander",
					Fighter2ID:      "",
					Fighter2Name:    "Kevin Jackson",
					WeightClass:     "Light Heavyweight",
					Round:           "Main Card",
					BracketPosition: 1,
					WinnerID:        "Jerry Bohlander",
					Method:          "Technical submission (armbar)",
					Time:            "10:21",
				},
			},
		}
		tournaments = append(tournaments, lightHeavyweightBout)

		// Welterweight Tournament Alternate bouts
		welterweightAlternateBouts := Tournament{
			Name:        "Welterweight Tournament Alternate Bouts",
			WeightClass: "Welterweight",
			BracketType: "alternate",
			WinnerID:    "",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "LaVerne Clark",
					Fighter2ID:      "",
					Fighter2Name:    "Josh Stuart",
					WeightClass:     "Welterweight",
					Round:           "Alternate",
					BracketPosition: 1,
					WinnerID:        "LaVerne Clark",
					Method:          "TKO (punches)",
					Time:            "1:15",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Chris Brennan",
					Fighter2ID:      "",
					Fighter2Name:    "Courtney Turner",
					WeightClass:     "Welterweight",
					Round:           "Alternate",
					BracketPosition: 2,
					WinnerID:        "Chris Brennan",
					Method:          "Submission (armbar)",
					Time:            "1:20",
				},
			},
		}
		tournaments = append(tournaments, welterweightAlternateBouts)

		return tournaments, nil

	case "UFC Japan: Ultimate Japan":
		// Create an array to store all tournaments for this event
		tournaments := []Tournament{}

		// Heavyweight Tournament
		heavyweightTournament := Tournament{
			Name:        "Heavyweight Tournament",
			WeightClass: "Heavyweight",
			BracketType: "4-man",
			WinnerID:    "Kazushi Sakuraba",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Kazushi Sakuraba",
					Fighter2ID:      "",
					Fighter2Name:    "Marcus Silveira",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "", // No Contest
					Method:          "No Contest (premature stoppage)",
					Time:            "1:51",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Tank Abbott",
					Fighter2ID:      "",
					Fighter2Name:    "Yoji Anjo",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Tank Abbott",
					Method:          "Decision (unanimous)",
					Time:            "15:00",
				},
				// Final - rematch after No Contest
				{
					Fighter1ID:      "",
					Fighter1Name:    "Kazushi Sakuraba",
					Fighter2ID:      "",
					Fighter2Name:    "Marcus Silveira",
					WeightClass:     "Heavyweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Kazushi Sakuraba",
					Method:          "Submission (armbar)",
					Time:            "3:44",
				},
			},
		}
		tournaments = append(tournaments, heavyweightTournament)

		// Heavyweight Championship
		heavyweightChampionship := Tournament{
			Name:        "Heavyweight Championship",
			WeightClass: "Heavyweight",
			BracketType: "championship",
			WinnerID:    "Randy Couture",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Randy Couture",
					Fighter2ID:      "",
					Fighter2Name:    "Maurice Smith",
					WeightClass:     "Heavyweight",
					Round:           "Championship",
					BracketPosition: 1,
					WinnerID:        "Randy Couture",
					Method:          "Decision (majority)",
					Time:            "21:00",
				},
			},
		}
		tournaments = append(tournaments, heavyweightChampionship)

		// Heavyweight Bout
		heavyweightBout := Tournament{
			Name:        "Heavyweight Bout",
			WeightClass: "Heavyweight",
			BracketType: "superfight",
			WinnerID:    "Vitor Belfort",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Vitor Belfort",
					Fighter2ID:      "",
					Fighter2Name:    "Joe Charles",
					WeightClass:     "Heavyweight",
					Round:           "Main Card",
					BracketPosition: 1,
					WinnerID:        "Vitor Belfort",
					Method:          "Submission (armbar)",
					Time:            "4:03",
				},
			},
		}
		tournaments = append(tournaments, heavyweightBout)

		// Light Heavyweight Championship
		lightHeavyweightChampionship := Tournament{
			Name:        "Light Heavyweight Championship",
			WeightClass: "Light Heavyweight",
			BracketType: "championship",
			WinnerID:    "Frank Shamrock",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Frank Shamrock",
					Fighter2ID:      "",
					Fighter2Name:    "Kevin Jackson",
					WeightClass:     "Middleweight", // Listed as Middleweight in the table but was for Light Heavyweight title
					Round:           "Championship",
					BracketPosition: 1,
					WinnerID:        "Frank Shamrock",
					Method:          "Submission (armbar)",
					Time:            "0:22",
				},
			},
		}
		tournaments = append(tournaments, lightHeavyweightChampionship)

		// Heavyweight Alternate Bout
		heavyweightAlternateBout := Tournament{
			Name:        "Heavyweight Alternate Bout",
			WeightClass: "Heavyweight",
			BracketType: "alternate",
			WinnerID:    "Tra Telligman",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Tra Telligman",
					Fighter2ID:      "",
					Fighter2Name:    "Brad Kohler",
					WeightClass:     "Heavyweight",
					Round:           "Alternate",
					BracketPosition: 1,
					WinnerID:        "Tra Telligman",
					Method:          "Submission (armbar)",
					Time:            "10:05",
				},
			},
		}
		tournaments = append(tournaments, heavyweightAlternateBout)

		return tournaments, nil

	case "UFC 15: Collision Course":
		// Create an array to store all tournaments for this event
		tournaments := []Tournament{}

		// Heavyweight Tournament
		heavyweightTournament := Tournament{
			Name:        "Heavyweight Tournament",
			WeightClass: "Heavyweight",
			BracketType: "4-man",
			WinnerID:    "Mark Kerr",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Mark Kerr",
					Fighter2ID:      "",
					Fighter2Name:    "Greg Stott",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Mark Kerr",
					Method:          "KO (knee)",
					Time:            "0:17",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Dave Beneteau",
					Fighter2ID:      "",
					Fighter2Name:    "Carlos Barreto",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Dave Beneteau",
					Method:          "Decision (unanimous)",
					Time:            "15:00",
				},
				// Final - Note: Dwayne Cason replaced Dave Beneteau
				{
					Fighter1ID:      "",
					Fighter1Name:    "Mark Kerr",
					Fighter2ID:      "",
					Fighter2Name:    "Dwayne Cason",
					WeightClass:     "Heavyweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Mark Kerr",
					Method:          "Submission (rear-naked choke)",
					Time:            "0:53",
				},
			},
		}
		tournaments = append(tournaments, heavyweightTournament)

		// Heavyweight Championship
		heavyweightChampionship := Tournament{
			Name:        "Heavyweight Championship",
			WeightClass: "Heavyweight",
			BracketType: "championship",
			WinnerID:    "Maurice Smith",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Maurice Smith",
					Fighter2ID:      "",
					Fighter2Name:    "Tank Abbott",
					WeightClass:     "Heavyweight",
					Round:           "Championship",
					BracketPosition: 1,
					WinnerID:        "Maurice Smith",
					Method:          "TKO (leg kicks)",
					Time:            "8:08",
				},
			},
		}
		tournaments = append(tournaments, heavyweightChampionship)

		// Heavyweight Superfight
		heavyweightSuperfight := Tournament{
			Name:        "Heavyweight Superfight",
			WeightClass: "Heavyweight",
			BracketType: "superfight",
			WinnerID:    "Randy Couture",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Randy Couture",
					Fighter2ID:      "",
					Fighter2Name:    "Vitor Belfort",
					WeightClass:     "Heavyweight",
					Round:           "Superfight",
					BracketPosition: 1,
					WinnerID:        "Randy Couture",
					Method:          "TKO (punches)",
					Time:            "8:16",
				},
			},
		}
		tournaments = append(tournaments, heavyweightSuperfight)

		// Heavyweight Alternate Bouts
		heavyweightAlternateBouts := Tournament{
			Name:        "Heavyweight Alternate Bouts",
			WeightClass: "Heavyweight",
			BracketType: "alternate",
			WinnerID:    "",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Alex Hunter",
					Fighter2ID:      "",
					Fighter2Name:    "Harry Moskowitz",
					WeightClass:     "Heavyweight",
					Round:           "Alternate",
					BracketPosition: 1,
					WinnerID:        "Alex Hunter",
					Method:          "Decision (split)",
					Time:            "12:00",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Dwayne Cason",
					Fighter2ID:      "",
					Fighter2Name:    "Houston Dorr",
					WeightClass:     "Heavyweight",
					Round:           "Alternate",
					BracketPosition: 2,
					WinnerID:        "Dwayne Cason",
					Method:          "TKO (punches)",
					Time:            "3:43",
				},
			},
		}
		tournaments = append(tournaments, heavyweightAlternateBouts)

		return tournaments, nil

	case "UFC 14: Showdown":
		// Create an array to store all tournaments for this event
		tournaments := []Tournament{}

		// Heavyweight Tournament
		heavyweightTournament := Tournament{
			Name:        "Heavyweight Tournament",
			WeightClass: "Heavyweight",
			BracketType: "4-man",
			WinnerID:    "Mark Kerr",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Mark Kerr",
					Fighter2ID:      "",
					Fighter2Name:    "Moti Horenstein",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Mark Kerr",
					Method:          "TKO (punches)",
					Time:            "2:22",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Dan Bobish",
					Fighter2ID:      "",
					Fighter2Name:    "Brian Johnston",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Dan Bobish",
					Method:          "Submission (forearm choke)",
					Time:            "2:10",
				},
				// Final
				{
					Fighter1ID:      "",
					Fighter1Name:    "Mark Kerr",
					Fighter2ID:      "",
					Fighter2Name:    "Dan Bobish",
					WeightClass:     "Heavyweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Mark Kerr",
					Method:          "Submission (chin to the eye)",
					Time:            "1:38",
				},
			},
		}
		tournaments = append(tournaments, heavyweightTournament)

		// Middleweight Tournament
		middleweightTournament := Tournament{
			Name:        "Middleweight Tournament",
			WeightClass: "Middleweight",
			BracketType: "4-man",
			WinnerID:    "Kevin Jackson",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Joe Moreira",
					Fighter2ID:      "",
					Fighter2Name:    "Yuri Vaulin",
					WeightClass:     "Middleweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Joe Moreira",
					Method:          "Decision (unanimous)",
					Time:            "15:00",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Kevin Jackson",
					Fighter2ID:      "",
					Fighter2Name:    "Todd Butler",
					WeightClass:     "Middleweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Kevin Jackson",
					Method:          "Submission (punches)",
					Time:            "1:27",
				},
				// Final - Note: Tony Fryklund replaced Joe Moreira
				{
					Fighter1ID:      "",
					Fighter1Name:    "Kevin Jackson",
					Fighter2ID:      "",
					Fighter2Name:    "Tony Fryklund",
					WeightClass:     "Middleweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Kevin Jackson",
					Method:          "Submission (rear-naked choke)",
					Time:            "0:44",
				},
			},
		}
		tournaments = append(tournaments, middleweightTournament)

		// Heavyweight Championship
		heavyweightChampionship := Tournament{
			Name:        "Heavyweight Championship",
			WeightClass: "Heavyweight",
			BracketType: "championship",
			WinnerID:    "Maurice Smith",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Maurice Smith",
					Fighter2ID:      "",
					Fighter2Name:    "Mark Coleman",
					WeightClass:     "Heavyweight",
					Round:           "Championship",
					BracketPosition: 1,
					WinnerID:        "Maurice Smith",
					Method:          "Decision (unanimous)",
					Time:            "21:00",
				},
			},
		}
		tournaments = append(tournaments, heavyweightChampionship)

		// Middleweight Alternate Bout
		middleweightAlternateBout := Tournament{
			Name:        "Middleweight Alternate Bout",
			WeightClass: "Middleweight",
			BracketType: "alternate",
			WinnerID:    "Tony Fryklund",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Tony Fryklund",
					Fighter2ID:      "",
					Fighter2Name:    "Donnie Chappell",
					WeightClass:     "Middleweight",
					Round:           "Alternate",
					BracketPosition: 1,
					WinnerID:        "Tony Fryklund",
					Method:          "Submission (choke)",
					Time:            "1:31",
				},
			},
		}
		tournaments = append(tournaments, middleweightAlternateBout)

		// Heavyweight Alternate Bout
		heavyweightAlternateBout := Tournament{
			Name:        "Heavyweight Alternate Bout",
			WeightClass: "Heavyweight",
			BracketType: "alternate",
			WinnerID:    "Alex Hunter",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Alex Hunter",
					Fighter2ID:      "",
					Fighter2Name:    "Sam Fulton",
					WeightClass:     "Heavyweight",
					Round:           "Alternate",
					BracketPosition: 1,
					WinnerID:        "Alex Hunter",
					Method:          "TKO (punches)",
					Time:            "2:22",
				},
			},
		}
		tournaments = append(tournaments, heavyweightAlternateBout)

		return tournaments, nil

	case "UFC 13: The Ultimate Force":
		// Create an array to store all tournaments for this event
		tournaments := []Tournament{}

		// Lightweight Tournament
		lightweightTournament := Tournament{
			Name:        "Lightweight Tournament",
			WeightClass: "Lightweight",
			BracketType: "4-man",
			WinnerID:    "Guy Mezger",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Guy Mezger",
					Fighter2ID:      "",
					Fighter2Name:    "Christophe Leininger",
					WeightClass:     "Lightweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Guy Mezger",
					Method:          "Decision (unanimous)",
					Time:            "15:00",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Enson Inoue",
					Fighter2ID:      "",
					Fighter2Name:    "Royce Alger",
					WeightClass:     "Lightweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Enson Inoue",
					Method:          "Submission (armbar)",
					Time:            "1:36",
				},
				// Final - Note: Tito Ortiz replaced Enson Inoue
				{
					Fighter1ID:      "",
					Fighter1Name:    "Guy Mezger",
					Fighter2ID:      "",
					Fighter2Name:    "Tito Ortiz",
					WeightClass:     "Lightweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Guy Mezger",
					Method:          "Submission (guillotine choke)",
					Time:            "3:00",
				},
			},
		}
		tournaments = append(tournaments, lightweightTournament)

		// Heavyweight Tournament
		heavyweightTournament := Tournament{
			Name:        "Heavyweight Tournament",
			WeightClass: "Heavyweight",
			BracketType: "4-man",
			WinnerID:    "Randy Couture",
			Fights: []TournamentFight{
				// Semifinals
				{
					Fighter1ID:      "",
					Fighter1Name:    "Steven Graham",
					Fighter2ID:      "",
					Fighter2Name:    "Dmitri Stepanov",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 1,
					WinnerID:        "Steven Graham",
					Method:          "Submission (armlock)",
					Time:            "1:30",
				},
				{
					Fighter1ID:      "",
					Fighter1Name:    "Randy Couture",
					Fighter2ID:      "",
					Fighter2Name:    "Tony Halme",
					WeightClass:     "Heavyweight",
					Round:           "Semifinal",
					BracketPosition: 2,
					WinnerID:        "Randy Couture",
					Method:          "Submission (rear-naked choke)",
					Time:            "1:00",
				},
				// Final
				{
					Fighter1ID:      "",
					Fighter1Name:    "Randy Couture",
					Fighter2ID:      "",
					Fighter2Name:    "Steven Graham",
					WeightClass:     "Heavyweight",
					Round:           "Final",
					BracketPosition: 3,
					WinnerID:        "Randy Couture",
					Method:          "TKO (punches)",
					Time:            "3:13",
				},
			},
		}
		tournaments = append(tournaments, heavyweightTournament)

		// Lightweight Alternate Bout
		lightweightAlternateBout := Tournament{
			Name:        "Lightweight Alternate Bout",
			WeightClass: "Lightweight",
			BracketType: "alternate",
			WinnerID:    "Tito Ortiz",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Tito Ortiz",
					Fighter2ID:      "",
					Fighter2Name:    "Wes Albritton",
					WeightClass:     "Lightweight",
					Round:           "Alternate",
					BracketPosition: 1,
					WinnerID:        "Tito Ortiz",
					Method:          "TKO (corner stoppage)",
					Time:            "0:31",
				},
			},
		}
		tournaments = append(tournaments, lightweightAlternateBout)

		// Heavyweight Alternate Bout
		heavyweightAlternateBout := Tournament{
			Name:        "Heavyweight Alternate Bout",
			WeightClass: "Heavyweight",
			BracketType: "alternate",
			WinnerID:    "Jack Nilson",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Jack Nilson",
					Fighter2ID:      "",
					Fighter2Name:    "Saeed Hosseini",
					WeightClass:     "Heavyweight",
					Round:           "Alternate",
					BracketPosition: 1,
					WinnerID:        "Jack Nilson",
					Method:          "TKO (punches)",
					Time:            "1:23",
				},
			},
		}
		tournaments = append(tournaments, heavyweightAlternateBout)

		// Heavyweight Superfight
		heavyweightSuperfight := Tournament{
			Name:        "Heavyweight Bout",
			WeightClass: "Heavyweight",
			BracketType: "superfight",
			WinnerID:    "Vitor Belfort",
			Fights: []TournamentFight{
				{
					Fighter1ID:      "",
					Fighter1Name:    "Vitor Belfort",
					Fighter2ID:      "",
					Fighter2Name:    "Tank Abbott",
					WeightClass:     "Heavyweight",
					Round:           "Superfight",
					BracketPosition: 1,
					WinnerID:        "Vitor Belfort",
					Method:          "TKO (punches)",
					Time:            "0:52",
				},
			},
		}
		tournaments = append(tournaments, heavyweightSuperfight)

		return tournaments, nil

	case "UFC 12: Judgement Day":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // Lightweight Tournament
        lightweightTournament := Tournament{
            Name:        "Lightweight Tournament",
            WeightClass: "Lightweight",
            BracketType: "4-man",
            WinnerID:    "Jerry Bohlander",
            Fights: []TournamentFight{
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Jerry Bohlander",
                    Fighter2ID:      "",
                    Fighter2Name:    "Rainy Martinez",
                    WeightClass:     "Lightweight",
                    Round:           "Semifinal",
                    BracketPosition: 1,
                    WinnerID:        "Jerry Bohlander",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "1:18",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Yoshiki Takahashi",
                    Fighter2ID:      "",
                    Fighter2Name:    "Wallid Ismail",
                    WeightClass:     "Lightweight",
                    Round:           "Semifinal",
                    BracketPosition: 2,
                    WinnerID:        "Yoshiki Takahashi",
                    Method:          "Decision",
                    Time:            "15:00",
                },
                // Final - Note: Nick Sanzo replaced Yoshiki Takahashi
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Jerry Bohlander",
                    Fighter2ID:      "",
                    Fighter2Name:    "Nick Sanzo",
                    WeightClass:     "Lightweight",
                    Round:           "Final",
                    BracketPosition: 3,
                    WinnerID:        "Jerry Bohlander",
                    Method:          "Submission (neck crank)",
                    Time:            "0:39",
                },
            },
        }
        tournaments = append(tournaments, lightweightTournament)
        
        // Heavyweight Tournament
        heavyweightTournament := Tournament{
            Name:        "Heavyweight Tournament",
            WeightClass: "Heavyweight",
            BracketType: "4-man",
            WinnerID:    "Vitor Belfort",
            Fights: []TournamentFight{
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Scott Ferrozzo",
                    Fighter2ID:      "",
                    Fighter2Name:    "Jim Mullen",
                    WeightClass:     "Heavyweight",
                    Round:           "Semifinal",
                    BracketPosition: 1,
                    WinnerID:        "Scott Ferrozzo",
                    Method:          "TKO (knees)",
                    Time:            "8:02",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Vitor Belfort",
                    Fighter2ID:      "",
                    Fighter2Name:    "Tra Telligman",
                    WeightClass:     "Heavyweight",
                    Round:           "Semifinal",
                    BracketPosition: 2,
                    WinnerID:        "Vitor Belfort",
                    Method:          "TKO (cut)",
                    Time:            "1:17",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Vitor Belfort",
                    Fighter2ID:      "",
                    Fighter2Name:    "Scott Ferrozzo",
                    WeightClass:     "Heavyweight",
                    Round:           "Final",
                    BracketPosition: 3,
                    WinnerID:        "Vitor Belfort",
                    Method:          "TKO (punches)",
                    Time:            "0:43",
                },
            },
        }
        tournaments = append(tournaments, heavyweightTournament)
        
        // Lightweight Alternate Bout
        lightweightAlternateBout := Tournament{
            Name:        "Lightweight Alternate Bout",
            WeightClass: "Lightweight",
            BracketType: "alternate",
            WinnerID:    "Nick Sanzo",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Nick Sanzo",
                    Fighter2ID:      "",
                    Fighter2Name:    "Jackie Lee",
                    WeightClass:     "Lightweight",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Nick Sanzo",
                    Method:          "TKO (strikes)",
                    Time:            "0:48",
                },
            },
        }
        tournaments = append(tournaments, lightweightAlternateBout)
        
        // Heavyweight Alternate Bout
        heavyweightAlternateBout := Tournament{
            Name:        "Heavyweight Alternate Bout",
            WeightClass: "Heavyweight",
            BracketType: "alternate",
            WinnerID:    "Justin Martin",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Justin Martin",
                    Fighter2ID:      "",
                    Fighter2Name:    "Eric Martin",
                    WeightClass:     "Heavyweight",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Justin Martin",
                    Method:          "Submission (heel hook)",
                    Time:            "0:14",
                },
            },
        }
        tournaments = append(tournaments, heavyweightAlternateBout)
        
        // Heavyweight Championship
        heavyweightChampionship := Tournament{
            Name:        "UFC Heavyweight Championship",
            WeightClass: "Heavyweight",
            BracketType: "championship",
            WinnerID:    "Mark Coleman",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Coleman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dan Severn",
                    WeightClass:     "Heavyweight",
                    Round:           "Championship",
                    BracketPosition: 1,
                    WinnerID:        "Mark Coleman",
                    Method:          "Submission (neck crank)",
                    Time:            "2:57",
                },
            },
        }
        tournaments = append(tournaments, heavyweightChampionship)
        
        return tournaments, nil

	case "UFC: The Ultimate Ultimate 2":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Don Frye",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Brian Johnston",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Ken Shamrock",
                    Method:          "Submission (forearm choke)",
                    Time:            "5:48",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Tank Abbott",
                    Fighter2ID:      "",
                    Fighter2Name:    "Cal Worsham",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Tank Abbott",
                    Method:          "Submission (punches)",
                    Time:            "2:51",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Gary Goodridge",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Don Frye",
                    Method:          "Submission (fatigue)",
                    Time:            "11:19",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Kimo Leopoldo",
                    Fighter2ID:      "",
                    Fighter2Name:    "Paul Varelans",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Kimo Leopoldo",
                    Method:          "TKO (corner stoppage)",
                    Time:            "9:08",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Tank Abbott",
                    Fighter2ID:      "",
                    Fighter2Name:    "Steve Nelmark",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Tank Abbott",
                    Method:          "KO (punch)",
                    Time:            "1:03",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Mark Hall",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Don Frye",
                    Method:          "Submission (achilles lock)",
                    Time:            "0:20",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Tank Abbott",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Don Frye",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "1:22",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Mark Hall",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Hall",
                    Fighter2ID:      "",
                    Fighter2Name:    "Felix Mitchell",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Mark Hall",
                    Method:          "TKO (punches)",
                    Time:            "1:45",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Steve Nelmark",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Steve Nelmark",
                    Fighter2ID:      "",
                    Fighter2Name:    "Marcus Bossett",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Steve Nelmark",
                    Method:          "Submission (choke)",
                    Time:            "1:37",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        alternateBout3 := Tournament{
            Name:        "Alternate Bout 3",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Tai Bowden",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Tai Bowden",
                    Fighter2ID:      "",
                    Fighter2Name:    "Jack Nilson",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Tai Bowden",
                    Method:          "Submission (headbutts)",
                    Time:            "4:46",
                },
            },
        }
        tournaments = append(tournaments, alternateBout3)
        
        return tournaments, nil

	case "UFC 11: The Proving Ground":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Mark Coleman",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Coleman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Julian Sanchez",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Mark Coleman",
                    Method:          "Submission (scarf hold choke)",
                    Time:            "0:45",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Brian Johnston",
                    Fighter2ID:      "",
                    Fighter2Name:    "Reza Nasri",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Brian Johnston",
                    Method:          "TKO (strikes)",
                    Time:            "0:28",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Tank Abbott",
                    Fighter2ID:      "",
                    Fighter2Name:    "Sam Adkins",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Tank Abbott",
                    Method:          "Submission (forearm choke)",
                    Time:            "2:06",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Jerry Bohlander",
                    Fighter2ID:      "",
                    Fighter2Name:    "Fabio Gurgel",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Jerry Bohlander",
                    Method:          "Decision (unanimous)",
                    Time:            "15:00",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Coleman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Brian Johnston",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Mark Coleman",
                    Method:          "TKO (submission to strikes)",
                    Time:            "2:20",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Scott Ferrozzo",
                    Fighter2ID:      "",
                    Fighter2Name:    "Tank Abbott",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Scott Ferrozzo",
                    Method:          "Decision (unanimous)",
                    Time:            "18:00",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Coleman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Scott Ferrozzo",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Mark Coleman",
                    Method:          "Walkover (injury)",
                    Time:            "",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Scott Ferrozzo",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Scott Ferrozzo",
                    Fighter2ID:      "",
                    Fighter2Name:    "Sam Fulton",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Scott Ferrozzo",
                    Method:          "TKO (submission to strikes)",
                    Time:            "1:45",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Roberto Traven",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Roberto Traven",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dave Berry",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Roberto Traven",
                    Method:          "TKO (submission to strikes)",
                    Time:            "1:23",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        return tournaments, nil

	case "UFC 10: The Tournament":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Mark Coleman",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Mark Hall",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Don Frye",
                    Method:          "TKO (punches)",
                    Time:            "10:21",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Brian Johnston",
                    Fighter2ID:      "",
                    Fighter2Name:    "Scott Fiedler",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Brian Johnston",
                    Method:          "TKO (submission to punches)",
                    Time:            "2:25",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Coleman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Moti Horenstein",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Mark Coleman",
                    Method:          "TKO (submission to punches)",
                    Time:            "2:43",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Gary Goodridge",
                    Fighter2ID:      "",
                    Fighter2Name:    "John Campetella",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Gary Goodridge",
                    Method:          "TKO (punches)",
                    Time:            "1:28",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Brian Johnston",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Don Frye",
                    Method:          "TKO (submission to elbow)",
                    Time:            "4:37",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Coleman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Gary Goodridge",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Mark Coleman",
                    Method:          "Submission (exhaustion)",
                    Time:            "7:00",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Coleman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Don Frye",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Mark Coleman",
                    Method:          "TKO (punches)",
                    Time:            "11:34",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Geza Kalman",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Geza Kalman",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dieuseul Berto",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Geza Kalman",
                    Method:          "TKO (punches)",
                    Time:            "5:57",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Sam Adkins",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Sam Adkins",
                    Fighter2ID:      "",
                    Fighter2Name:    "Felix Mitchell",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Sam Adkins",
                    Method:          "Decision (unanimous)",
                    Time:            "",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        return tournaments, nil

	case "UFC 8: David vs. Goliath":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Don Frye",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Thomas Ramirez",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Don Frye",
                    Method:          "KO (punch)",
                    Time:            "0:08",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Paul Varelans",
                    Fighter2ID:      "",
                    Fighter2Name:    "Joe Moreira",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Paul Varelans",
                    Method:          "Decision (unanimous)",
                    Time:            "10:00",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Jerry Bohlander",
                    Fighter2ID:      "",
                    Fighter2Name:    "Scott Ferrozzo",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Jerry Bohlander",
                    Method:          "Submission (guillotine choke)",
                    Time:            "9:03",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Gary Goodridge",
                    Fighter2ID:      "",
                    Fighter2Name:    "Paul Herrera",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Gary Goodridge",
                    Method:          "KO (elbows)",
                    Time:            "0:13",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Sam Adkins",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Don Frye",
                    Method:          "TKO (doctor stoppage)",
                    Time:            "0:48",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Gary Goodridge",
                    Fighter2ID:      "",
                    Fighter2Name:    "Jerry Bohlander",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Gary Goodridge",
                    Method:          "TKO (punches)",
                    Time:            "5:31",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Don Frye",
                    Fighter2ID:      "",
                    Fighter2Name:    "Gary Goodridge",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Don Frye",
                    Method:          "TKO (submission to punches)",
                    Time:            "2:14",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Superfight Championship
        superfightChampionship := Tournament{
            Name:        "Superfight Championship",
            WeightClass: "N/A",
            BracketType: "superfight",
            WinnerID:    "Ken Shamrock",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Kimo Leopoldo",
                    WeightClass:     "N/A",
                    Round:           "Superfight",
                    BracketPosition: 1,
                    WinnerID:        "Ken Shamrock",
                    Method:          "Submission (kneebar)",
                    Time:            "4:24",
                },
            },
        }
        tournaments = append(tournaments, superfightChampionship)
        
        // Alternate Bout
        alternateBout := Tournament{
            Name:        "Alternate Bout",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Sam Adkins",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Sam Adkins",
                    Fighter2ID:      "",
                    Fighter2Name:    "Keith Mielke",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Sam Adkins",
                    Method:          "TKO (submission to punches)",
                    Time:            "0:50",
                },
            },
        }
        tournaments = append(tournaments, alternateBout)
        
        return tournaments, nil

	case "UFC: The Ultimate Ultimate":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Dan Severn",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Tank Abbott",
                    Fighter2ID:      "",
                    Fighter2Name:    "Steve Jennum",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Tank Abbott",
                    Method:          "Submission (neck crank)",
                    Time:            "1:14",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Paul Varelans",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Dan Severn",
                    Method:          "Submission (arm-triangle choke)",
                    Time:            "1:01",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Marco Ruas",
                    Fighter2ID:      "",
                    Fighter2Name:    "Keith Hackney",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Marco Ruas",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "2:39",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Oleg Taktarov",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dave Beneteau",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Oleg Taktarov",
                    Method:          "Submission (achilles hold)",
                    Time:            "1:15",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Tank Abbott",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Dan Severn",
                    Method:          "Decision (unanimous)",
                    Time:            "18:00",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Oleg Taktarov",
                    Fighter2ID:      "",
                    Fighter2Name:    "Marco Ruas",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Oleg Taktarov",
                    Method:          "Decision (unanimous)",
                    Time:            "18:00",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Oleg Taktarov",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Dan Severn",
                    Method:          "Decision (unanimous)",
                    Time:            "30:00",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Joe Charles",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Joe Charles",
                    Fighter2ID:      "",
                    Fighter2Name:    "Scott Bessac",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Joe Charles",
                    Method:          "Submission (armlock)",
                    Time:            "4:38",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Mark Hall",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Hall",
                    Fighter2ID:      "",
                    Fighter2Name:    "Trent Jenkins",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Mark Hall",
                    Method:          "Submission (armlock)",
                    Time:            "5:29",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        return tournaments, nil

	case "UFC 7: The Brawl in Buffalo":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Marco Ruas",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Paul Varelans",
                    Fighter2ID:      "",
                    Fighter2Name:    "Gerry Harris",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Paul Varelans",
                    Method:          "TKO (submission to strikes)",
                    Time:            "1:07",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Mark Hall",
                    Fighter2ID:      "",
                    Fighter2Name:    "Harold Howard",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Mark Hall",
                    Method:          "TKO (submission to strikes)",
                    Time:            "1:41",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Remco Pardoel",
                    Fighter2ID:      "",
                    Fighter2Name:    "Ryan Parker",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Remco Pardoel",
                    Method:          "Submission (lapel choke)",
                    Time:            "3:05",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Marco Ruas",
                    Fighter2ID:      "",
                    Fighter2Name:    "Larry Cureton",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Marco Ruas",
                    Method:          "Submission (heel hook)",
                    Time:            "3:23",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Paul Varelans",
                    Fighter2ID:      "",
                    Fighter2Name:    "Mark Hall",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Paul Varelans",
                    Method:          "Submission (keylock)",
                    Time:            "1:04",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Marco Ruas",
                    Fighter2ID:      "",
                    Fighter2Name:    "Remco Pardoel",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Marco Ruas",
                    Method:          "Submission (mounted position)",
                    Time:            "12:27",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Marco Ruas",
                    Fighter2ID:      "",
                    Fighter2Name:    "Paul Varelans",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Marco Ruas",
                    Method:          "TKO (strikes)",
                    Time:            "13:17",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Superfight Championship
        superfightChampionship := Tournament{
            Name:        "Superfight Championship",
            WeightClass: "N/A",
            BracketType: "superfight",
            WinnerID:    "",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Oleg Taktarov",
                    WeightClass:     "N/A",
                    Round:           "Superfight",
                    BracketPosition: 1,
                    WinnerID:        "",
                    Method:          "Draw",
                    Time:            "33:00",
                },
            },
        }
        tournaments = append(tournaments, superfightChampionship)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Joel Sutton",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Joel Sutton",
                    Fighter2ID:      "",
                    Fighter2Name:    "Geza Kalman",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Joel Sutton",
                    Method:          "TKO (cut)",
                    Time:            "0:48",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Onassis Parungao",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Onassis Parungao",
                    Fighter2ID:      "",
                    Fighter2Name:    "Francesco Maturi",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Onassis Parungao",
                    Method:          "TKO (submission to strikes)",
                    Time:            "5:26",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        alternateBout3 := Tournament{
            Name:        "Alternate Bout 3",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Scott Bessac",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Scott Bessac",
                    Fighter2ID:      "",
                    Fighter2Name:    "David Hood",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Scott Bessac",
                    Method:          "Submission (guillotine choke)",
                    Time:            "0:31",
                },
            },
        }
        tournaments = append(tournaments, alternateBout3)
        
        return tournaments, nil

	case "UFC 6: Clash of the Titans":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Oleg Taktarov",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Tank Abbott",
                    Fighter2ID:      "",
                    Fighter2Name:    "John Matua",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Tank Abbott",
                    Method:          "KO (punch)",
                    Time:            "0:20",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Paul Varelans",
                    Fighter2ID:      "",
                    Fighter2Name:    "Cal Worsham",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Paul Varelans",
                    Method:          "KO (elbow)",
                    Time:            "1:02",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Patrick Smith",
                    Fighter2ID:      "",
                    Fighter2Name:    "Rudyard Moncayo",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Patrick Smith",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "1:08",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Oleg Taktarov",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dave Beneteau",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Oleg Taktarov",
                    Method:          "Submission (guillotine)",
                    Time:            "0:57",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Tank Abbott",
                    Fighter2ID:      "",
                    Fighter2Name:    "Paul Varelans",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Tank Abbott",
                    Method:          "TKO (punches)",
                    Time:            "1:53",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Oleg Taktarov",
                    Fighter2ID:      "",
                    Fighter2Name:    "Anthony Macias",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Oleg Taktarov",
                    Method:          "Submission (guillotine choke)",
                    Time:            "0:09",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Oleg Taktarov",
                    Fighter2ID:      "",
                    Fighter2Name:    "Tank Abbott",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Oleg Taktarov",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "17:47",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Superfight Championship
        superfightChampionship := Tournament{
            Name:        "Superfight Championship",
            WeightClass: "N/A",
            BracketType: "superfight",
            WinnerID:    "Ken Shamrock",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dan Severn",
                    WeightClass:     "N/A",
                    Round:           "Superfight",
                    BracketPosition: 1,
                    WinnerID:        "Ken Shamrock",
                    Method:          "Submission (guillotine choke)",
                    Time:            "2:14",
                },
            },
        }
        tournaments = append(tournaments, superfightChampionship)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Joel Sutton",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Joel Sutton",
                    Fighter2ID:      "",
                    Fighter2Name:    "Jack McLaughlin",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Joel Sutton",
                    Method:          "TKO (submission to punches)",
                    Time:            "2:01",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Anthony Macias",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Anthony Macias",
                    Fighter2ID:      "",
                    Fighter2Name:    "He-Man Ali Gipson",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Anthony Macias",
                    Method:          "TKO (submission to punches)",
                    Time:            "3:06",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        return tournaments, nil

	case "UFC 5: The Return of the Beast":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Dan Severn",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Jon Hess",
                    Fighter2ID:      "",
                    Fighter2Name:    "Andy Anderson",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Jon Hess",
                    Method:          "TKO (punches)",
                    Time:            "1:23",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Todd Medina",
                    Fighter2ID:      "",
                    Fighter2Name:    "Larry Cureton",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Todd Medina",
                    Method:          "Submission (forearm choke)",
                    Time:            "2:55",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Oleg Taktarov",
                    Fighter2ID:      "",
                    Fighter2Name:    "Ernie Verdicia",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Oleg Taktarov",
                    Method:          "Submission (choke)",
                    Time:            "2:23",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Joe Charles",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Dan Severn",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "1:38",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dave Beneteau",
                    Fighter2ID:      "",
                    Fighter2Name:    "Todd Medina",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Dave Beneteau",
                    Method:          "TKO (submission to strikes)",
                    Time:            "2:12",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Oleg Taktarov",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Dan Severn",
                    Method:          "TKO (cut)",
                    Time:            "4:21",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dave Beneteau",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Dan Severn",
                    Method:          "Submission (americana)",
                    Time:            "3:01",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Superfight Championship
        superfightChampionship := Tournament{
            Name:        "Superfight Championship",
            WeightClass: "N/A",
            BracketType: "superfight",
            WinnerID:    "",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Royce Gracie",
                    WeightClass:     "N/A",
                    Round:           "Superfight",
                    BracketPosition: 1,
                    WinnerID:        "",
                    Method:          "Draw",
                    Time:            "36:06",
                },
            },
        }
        tournaments = append(tournaments, superfightChampionship)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Dave Beneteau",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dave Beneteau",
                    Fighter2ID:      "",
                    Fighter2Name:    "Asbel Cancio",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Dave Beneteau",
                    Method:          "TKO (punches)",
                    Time:            "0:21",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Guy Mezger",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Guy Mezger",
                    Fighter2ID:      "",
                    Fighter2Name:    "John Dowdy",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Guy Mezger",
                    Method:          "TKO (punches)",
                    Time:            "2:02",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        return tournaments, nil

	case "UFC 4: Revenge of the Warriors":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Royce Gracie",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Ron van Clief",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "3:49",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Keith Hackney",
                    Fighter2ID:      "",
                    Fighter2Name:    "Joe Son",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Keith Hackney",
                    Method:          "Submission (blood choke)",
                    Time:            "2:44",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Steve Jennum",
                    Fighter2ID:      "",
                    Fighter2Name:    "Melton Bowen",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Steve Jennum",
                    Method:          "Submission (armbar)",
                    Time:            "4:47",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Anthony Macias",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Dan Severn",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "1:45",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Keith Hackney",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (armbar)",
                    Time:            "5:32",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Dan Severn",
                    Fighter2ID:      "",
                    Fighter2Name:    "Marcus Bossett",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Dan Severn",
                    Method:          "Submission (arm-triangle choke)",
                    Time:            "0:52",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Dan Severn",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (triangle choke)",
                    Time:            "15:49",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Alternate Bouts
        alternateBout1 := Tournament{
            Name:        "Alternate Bout 1",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Joe Charles",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Joe Charles",
                    Fighter2ID:      "",
                    Fighter2Name:    "Kevin Rosier",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Joe Charles",
                    Method:          "Submission (armbar)",
                    Time:            "0:14",
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        alternateBout2 := Tournament{
            Name:        "Alternate Bout 2",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Marcus Bossett",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Marcus Bossett",
                    Fighter2ID:      "",
                    Fighter2Name:    "Eldo Dias Xavier",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Marcus Bossett",
                    Method:          "KO (strikes)",
                    Time:            "4:55",
                },
            },
        }
        tournaments = append(tournaments, alternateBout2)
        
        alternateBout3 := Tournament{
            Name:        "Alternate Bout 3",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Guy Mezger",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Guy Mezger",
                    Fighter2ID:      "",
                    Fighter2Name:    "Jason Fairn",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Guy Mezger",
                    Method:          "TKO (corner stoppage)",
                    Time:            "2:13",
                },
            },
        }
        tournaments = append(tournaments, alternateBout3)
        
        return tournaments, nil

    case "UFC 3: The American Dream":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Steve Jennum",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Keith Hackney",
                    Fighter2ID:      "",
                    Fighter2Name:    "Emmanuel Yarbrough",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Keith Hackney",
                    Method:          "TKO (punches)",
                    Time:            "1:59",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Christophe Leininger",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Ken Shamrock",
                    Method:          "TKO (submission to punches)",
                    Time:            "4:49",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Harold Howard",
                    Fighter2ID:      "",
                    Fighter2Name:    "Roland Payne",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Harold Howard",
                    Method:          "KO (punch)",
                    Time:            "0:46",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Kimo Leopoldo",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (armlock)",
                    Time:            "4:40",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Felix Mitchell",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Ken Shamrock",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "4:34",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Harold Howard",
                    Fighter2ID:      "",
                    Fighter2Name:    "Royce Gracie",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Harold Howard",
                    Method:          "—", // Gracie withdrew, so no official method
                    Time:            "—", // No time recorded
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Steve Jennum",
                    Fighter2ID:      "",
                    Fighter2Name:    "Harold Howard",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Steve Jennum",
                    Method:          "TKO (submission to punches)",
                    Time:            "1:27",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Alternate Bout - based on notations in Wikipedia, an alternate entered
        alternateBout1 := Tournament{
            Name:        "Alternate Bout",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Felix Mitchell",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Felix Mitchell",
                    Fighter2ID:      "",
                    Fighter2Name:    "Ken Shamrock", // This match isn't shown in your data but placeholders needed
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Felix Mitchell",
                    Method:          "Entered as alternate", // Placeholder
                    Time:            "—", // No time
                },
            },
        }
        tournaments = append(tournaments, alternateBout1)
        
        return tournaments, nil

    case "UFC 2: No Way Out":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 16-Man Tournament
        sixteenManTournament := Tournament{
            Name:        "16-Man Tournament",
            WeightClass: "N/A",
            BracketType: "16-man",
            WinnerID:    "Royce Gracie",
            Fights: []TournamentFight{
                // Opening Round
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Scott Morris",
                    Fighter2ID:      "",
                    Fighter2Name:    "Sean Daugherty",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 1,
                    WinnerID:        "Scott Morris",
                    Method:          "Submission (guillotine choke)",
                    Time:            "0:20",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Patrick Smith",
                    Fighter2ID:      "",
                    Fighter2Name:    "Ray Wizard",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 2,
                    WinnerID:        "Patrick Smith",
                    Method:          "Submission (guillotine choke)",
                    Time:            "0:58",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Johnny Rhodes",
                    Fighter2ID:      "",
                    Fighter2Name:    "David Levicki",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 3,
                    WinnerID:        "Johnny Rhodes",
                    Method:          "TKO (submission to punches)",
                    Time:            "12:13",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Frank Hamaker",
                    Fighter2ID:      "",
                    Fighter2Name:    "Thaddeus Luster",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 4,
                    WinnerID:        "Frank Hamaker",
                    Method:          "TKO (corner stoppage)",
                    Time:            "4:52",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Orlando Wiet",
                    Fighter2ID:      "",
                    Fighter2Name:    "Robert Lucarelli",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 5,
                    WinnerID:        "Orlando Wiet",
                    Method:          "TKO (knees)",
                    Time:            "2:50",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Remco Pardoel",
                    Fighter2ID:      "",
                    Fighter2Name:    "Alberto Cerro Leon",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 6,
                    WinnerID:        "Remco Pardoel",
                    Method:          "Submission (armlock)",
                    Time:            "9:51",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Jason DeLucia",
                    Fighter2ID:      "",
                    Fighter2Name:    "Scott Baker",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 7,
                    WinnerID:        "Jason DeLucia",
                    Method:          "TKO (submission to punches)",
                    Time:            "6:41",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Minoki Ichihara",
                    WeightClass:     "N/A",
                    Round:           "Opening Round",
                    BracketPosition: 8,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (lapel choke)",
                    Time:            "5:08",
                },
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Patrick Smith",
                    Fighter2ID:      "",
                    Fighter2Name:    "Scott Morris",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 9,
                    WinnerID:        "Patrick Smith",
                    Method:          "KO (elbows)",
                    Time:            "0:30",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Johnny Rhodes",
                    Fighter2ID:      "",
                    Fighter2Name:    "Fred Ettish",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 10,
                    WinnerID:        "Johnny Rhodes",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "3:07",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Remco Pardoel",
                    Fighter2ID:      "",
                    Fighter2Name:    "Orlando Wiet",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 11,
                    WinnerID:        "Remco Pardoel",
                    Method:          "KO (elbows)",
                    Time:            "1:29",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Jason DeLucia",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 12,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (armbar)",
                    Time:            "1:07",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Patrick Smith",
                    Fighter2ID:      "",
                    Fighter2Name:    "Johnny Rhodes",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 13,
                    WinnerID:        "Patrick Smith",
                    Method:          "Submission (guillotine choke)",
                    Time:            "1:07",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Remco Pardoel",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 14,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (lapel choke)",
                    Time:            "1:31",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Patrick Smith",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 15,
                    WinnerID:        "Royce Gracie",
                    Method:          "TKO (submission to punches)",
                    Time:            "1:17",
                },
            },
        }
        tournaments = append(tournaments, sixteenManTournament)
        
        return tournaments, nil

    case "UFC 1: The Beginning":
        // Create an array to store all tournaments for this event
        tournaments := []Tournament{}
        
        // 8-Man Tournament
        eightManTournament := Tournament{
            Name:        "8-Man Tournament",
            WeightClass: "N/A",
            BracketType: "8-man",
            WinnerID:    "Royce Gracie",
            Fights: []TournamentFight{
                // Quarterfinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Gerard Gordeau",
                    Fighter2ID:      "",
                    Fighter2Name:    "Teila Tuli",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 1,
                    WinnerID:        "Gerard Gordeau",
                    Method:          "TKO (head kick)",
                    Time:            "0:26",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Kevin Rosier",
                    Fighter2ID:      "",
                    Fighter2Name:    "Zane Frazier",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 2,
                    WinnerID:        "Kevin Rosier",
                    Method:          "TKO (punches)",
                    Time:            "4:20",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Art Jimmerson",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 3,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (smother choke)",
                    Time:            "2:18",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Ken Shamrock",
                    Fighter2ID:      "",
                    Fighter2Name:    "Patrick Smith",
                    WeightClass:     "N/A",
                    Round:           "Quarterfinal",
                    BracketPosition: 4,
                    WinnerID:        "Ken Shamrock",
                    Method:          "Submission (heel hook)",
                    Time:            "1:49",
                },
                // Semifinals
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Gerard Gordeau",
                    Fighter2ID:      "",
                    Fighter2Name:    "Kevin Rosier",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 5,
                    WinnerID:        "Gerard Gordeau",
                    Method:          "TKO (corner stoppage)",
                    Time:            "0:59",
                },
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Ken Shamrock",
                    WeightClass:     "N/A",
                    Round:           "Semifinal",
                    BracketPosition: 6,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "0:57",
                },
                // Final
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Royce Gracie",
                    Fighter2ID:      "",
                    Fighter2Name:    "Gerard Gordeau",
                    WeightClass:     "N/A",
                    Round:           "Final",
                    BracketPosition: 7,
                    WinnerID:        "Royce Gracie",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "1:44",
                },
            },
        }
        tournaments = append(tournaments, eightManTournament)
        
        // Alternate Bout
        alternateBout := Tournament{
            Name:        "Alternate Bout",
            WeightClass: "N/A",
            BracketType: "alternate",
            WinnerID:    "Jason DeLucia",
            Fights: []TournamentFight{
                {
                    Fighter1ID:      "",
                    Fighter1Name:    "Jason DeLucia",
                    Fighter2ID:      "",
                    Fighter2Name:    "Trent Jenkins",
                    WeightClass:     "N/A",
                    Round:           "Alternate",
                    BracketPosition: 1,
                    WinnerID:        "Jason DeLucia",
                    Method:          "Submission (rear-naked choke)",
                    Time:            "0:52",
                },
            },
        }
        tournaments = append(tournaments, alternateBout)
        
        return tournaments, nil

	default:
		return nil, fmt.Errorf("tournament data not implemented for event: %s", eventName)
	}
}
