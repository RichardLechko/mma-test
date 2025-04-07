package scrapers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// UFCScrapedFight represents a fight scraped from UFC website
type UFCScrapedFight struct {
	Fighter1Name      string
	Fighter1GivenName string
	Fighter1LastName  string
	Fighter1Rank      string
	Fighter1Result    string
	Fighter2Name      string
	Fighter2GivenName string
	Fighter2LastName  string
	Fighter2Rank      string
	Fighter2Result    string
	WeightClass       string
	Method            string
	Round             string
	Time              string
	IsMainEvent       bool
	IsTitleFight      bool
}

// UFCFightScraper handles scraping fight data from UFC website
type UFCFightScraper struct {
	client *http.Client
}

// NewUFCFightScraper creates a new UFCFightScraper
func NewUFCFightScraper() *UFCFightScraper {
	return &UFCFightScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ScrapeFights retrieves all fights from a UFC event page
func (s *UFCFightScraper) ScrapeFights(ufcURL string) ([]UFCScrapedFight, error) {
	// Make HTTP request
	resp, err := s.client.Get(ufcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch UFC page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	// Parse HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var fights []UFCScrapedFight

	// Find all fight listings
	doc.Find(".c-listing-fight").Each(func(i int, fightElement *goquery.Selection) {
		var fight UFCScrapedFight

		// Weight class
		weightClass := fightElement.Find(".c-listing-fight__class-text").First().Text()
		fight.WeightClass = cleanText(weightClass)

		// Check if title fight
		fight.IsTitleFight = strings.Contains(strings.ToLower(weightClass), "title")

		// Main event is typically the first fight
		fight.IsMainEvent = i == 0

		// Get fighter 1 (red corner) details
		redCornerName := fightElement.Find(".c-listing-fight__corner-name--red")

		// Check if the fighter name is split into given/family name parts
		redGivenName := redCornerName.Find(".c-listing-fight__corner-given-name").Text()
		redFamilyName := redCornerName.Find(".c-listing-fight__corner-family-name").Text()

		if redGivenName != "" && redFamilyName != "" {
			fight.Fighter1GivenName = cleanText(redGivenName)
			fight.Fighter1LastName = cleanText(redFamilyName)
			fight.Fighter1Name = fight.Fighter1GivenName + " " + fight.Fighter1LastName
		} else {
			// If not split, get the full name
			fight.Fighter1Name = cleanText(redCornerName.Text())

			// Try to split the name if possible
			nameParts := strings.Fields(fight.Fighter1Name)
			if len(nameParts) >= 2 {
				fight.Fighter1GivenName = strings.Join(nameParts[:len(nameParts)-1], " ")
				fight.Fighter1LastName = nameParts[len(nameParts)-1]
			}
		}

		// Get fighter 2 (blue corner) details
		blueCornerName := fightElement.Find(".c-listing-fight__corner-name--blue")

		// Check if the fighter name is split into given/family name parts
		blueGivenName := blueCornerName.Find(".c-listing-fight__corner-given-name").Text()
		blueFamilyName := blueCornerName.Find(".c-listing-fight__corner-family-name").Text()

		if blueGivenName != "" && blueFamilyName != "" {
			fight.Fighter2GivenName = cleanText(blueGivenName)
			fight.Fighter2LastName = cleanText(blueFamilyName)
			fight.Fighter2Name = fight.Fighter2GivenName + " " + fight.Fighter2LastName
		} else {
			// If not split, get the full name
			fight.Fighter2Name = cleanText(blueCornerName.Text())

			// Try to split the name if possible
			nameParts := strings.Fields(fight.Fighter2Name)
			if len(nameParts) >= 2 {
				fight.Fighter2GivenName = strings.Join(nameParts[:len(nameParts)-1], " ")
				fight.Fighter2LastName = nameParts[len(nameParts)-1]
			}
		}

		// Get fighter ranks
		ranks := fightElement.Find(".c-listing-fight__corner-rank")
		if ranks.Length() >= 2 {
			// First rank is for red corner, second for blue corner
			fight.Fighter1Rank = cleanText(ranks.Eq(0).Text())
			fight.Fighter2Rank = cleanText(ranks.Eq(1).Text())
		}

		// Fight results
		fight.Round = cleanText(fightElement.Find(".c-listing-fight__result-text.round").Text())
		fight.Time = cleanText(fightElement.Find(".c-listing-fight__result-text.time").Text())
		fight.Method = cleanText(fightElement.Find(".c-listing-fight__result-text.method").Text())

		// Fight outcomes
		fight.Fighter1Result = cleanText(fightElement.Find(".c-listing-fight__corner--red .c-listing-fight__outcome-wrapper").Text())
		fight.Fighter2Result = cleanText(fightElement.Find(".c-listing-fight__corner--blue .c-listing-fight__outcome-wrapper").Text())

		// Only add the fight if we have both fighter names
		if fight.Fighter1Name != "" && fight.Fighter2Name != "" &&
			!strings.Contains(fight.Fighter1Name, "vs") &&
			!strings.Contains(fight.Fighter2Name, "vs") {
			fights = append(fights, fight)
		}
	})

	return fights, nil
}

// cleanText removes extra whitespace and normalizes text
func cleanText(text string) string {
	// Remove newlines and tabs
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Replace multiple spaces with a single space
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return strings.TrimSpace(text)
}
