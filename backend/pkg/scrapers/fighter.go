package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CountryCode represents a country in the UFC system
type CountryCode struct {
	Code         string
	Name         string
	FighterCount int
}

// FighterScraper with nationality support
type FighterScraper struct {
	*BaseScraper
	ufcAthleteListURL string
	ufcRankingsURL    string
	countryCodes      map[string]string // Maps country code to country name
}

func NewFighterScraper(config ScraperConfig) *FighterScraper {
	return &FighterScraper{
		BaseScraper:       NewBaseScraper(config),
		ufcAthleteListURL: "https://www.ufc.com/athletes/all",
		ufcRankingsURL:    "https://www.ufc.com/rankings",
		countryCodes:      initializeCountryCodes(),
	}
}

// Initialize the map of country codes to names
func initializeCountryCodes() map[string]string {
	return map[string]string{
		"AL": "Albania",
		"AR": "Argentina",
		"AM": "Armenia",
		"AU": "Australia",
		"AT": "Austria",
		"AZ": "Azerbaijan",
		"BH": "Bahrain",
		"BE": "Belgium",
		"BO": "Bolivia",
		"BR": "Brazil",
		"BG": "Bulgaria",
		"CM": "Cameroon",
		"CA": "Canada",
		"CL": "Chile",
		"CN": "China",
		"CO": "Colombia",
		"CR": "Costa Rica",
		"HR": "Croatia",
		"CZ": "Czechia",
		"CD": "Democratic Republic of the Congo",
		"DK": "Denmark",
		"EC": "Ecuador",
		"EN": "England",
		"FI": "Finland",
		"FR": "France",
		"GE": "Georgia",
		"DE": "Germany",
		"GR": "Greece",
		"GU": "Guam",
		"HK": "Hong Kong",
		"HU": "Hungary",
		"IS": "Iceland",
		"IN": "India",
		"ID": "Indonesia",
		"IE": "Ireland",
		"IT": "Italy",
		"JP": "Japan",
		"JO": "Jordan",
		"KZ": "Kazakhstan",
		"KG": "Kyrgyzstan",
		"LT": "Lithuania",
		"MX": "Mexico",
		"MD": "Moldova",
		"MN": "Mongolia",
		"MA": "Morocco",
		"NL": "Netherlands",
		"NZ": "New Zealand",
		"NO": "Norway",
		"PA": "Panama",
		"PY": "Paraguay",
		"PE": "Peru",
		"PH": "Philippines",
		"PL": "Poland",
		"PT": "Portugal",
		"RO": "Romania",
		"RU": "Russia",
		"SF": "Scotland",
		"RS": "Serbia",
		"SG": "Singapore",
		"SK": "Slovakia",
		"ZA": "South Africa",
		"KR": "South Korea",
		"ES": "Spain",
		"SR": "Suriname",
		"SE": "Sweden",
		"CH": "Switzerland",
		"TW": "Taiwan",
		"TJ": "Tajikistan",
		"TH": "Thailand",
		"TR": "Türkiye",
		"UA": "Ukraine",
		"AE": "United Arab Emirates",
		"GB": "United Kingdom",
		"US": "United States",
		"UY": "Uruguay",
		"UZ": "Uzbekistan",
		"VE": "Venezuela",
		"WL": "Wales",
	}
}

// Get total athlete count from the UI
func (s *FighterScraper) GetTotalAthleteCount() (int, error) {
	resp, err := s.client.Get(s.ufcAthleteListURL)
	if err != nil {
		return 0, fmt.Errorf("error fetching main page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error parsing HTML: %v", err)
	}

	// Extract total athlete count
	athleteCountText := strings.TrimSpace(doc.Find("div.althelete-total").Text())

	// Use regex to extract the number
	re := regexp.MustCompile(`(\d+)`)
	matches := re.FindStringSubmatch(athleteCountText)

	if len(matches) < 2 {
		return 0, fmt.Errorf("couldn't parse athlete count from: %s", athleteCountText)
	}

	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("error converting athlete count to number: %v", err)
	}

	log.Printf("Total athletes according to UFC site: %d", count)
	return count, nil
}

// Get all available country codes and fighter counts from the facet list
func (s *FighterScraper) GetAvailableCountries() ([]CountryCode, error) {
	var countries []CountryCode

	resp, err := s.client.Get(s.ufcAthleteListURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching main page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	// Find the facet list for countries
	doc.Find("ul[data-drupal-facet-id='athletes_residence_country_code'] li.facet-item").Each(func(i int, item *goquery.Selection) {
		linkElem := item.Find("a")
		if linkElem.Length() == 0 {
			return
		}

		// Extract the country code from the URL
		href, exists := linkElem.Attr("href")
		if !exists {
			return
		}

		// Parse out the country code from URL like "/athletes/all?filters%5B0%5D=location%3AUS"
		parts := strings.Split(href, "location%3A")
		if len(parts) < 2 {
			return
		}
		code := parts[1]

		// Extract the country name and fighter count
		countText := strings.TrimSpace(linkElem.Find("span.facet-item__value").Text())

		// Extract fighter count from data attribute
		countStr, exists := linkElem.Attr("data-drupal-facet-item-count")
		if !exists {
			countStr = "0"
		}

		count := 0
		fmt.Sscanf(countStr, "%d", &count)

		// Add to our list if we have a valid code and name
		if code != "" && countText != "" {
			countries = append(countries, CountryCode{
				Code:         code,
				Name:         countText,
				FighterCount: count,
			})
		}
	})

	log.Printf("Found %d countries in the facet list", len(countries))
	return countries, nil
}

// Modified Fighter struct with Nationality
type Fighter struct {
	Name        string
	Nickname    string
	WeightClass string
	Status      string
	Ranking     string
	Rankings    []FighterRanking
	Record      string
	UFCID       string
	UFCURL      string
	Nationality string

	Age    int
	Height string
	Weight string
	Reach  string

	KOWins  int
	SubWins int
	DecWins int
}

// FighterRanking represents a single ranking in a specific weight class
type FighterRanking struct {
	WeightClass string
	Rank        string
}

// Update the ScrapeFightersByCountry function to not skip "-0" entries
func (s *FighterScraper) ScrapeFightersByCountry(countryCode string) ([]*Fighter, error) {
	var fighters []*Fighter
	countryName, exists := s.countryCodes[countryCode]
	if !exists {
		return nil, fmt.Errorf("unknown country code: %s", countryCode)
	}

	// URL for this country's fighters
	countryURL := fmt.Sprintf("%s?filters%%5B0%%5D=location%%3A%s", s.ufcAthleteListURL, countryCode)
	log.Printf("Scraping fighters from %s (%s)", countryName, countryCode)

	// Track unique fighter IDs to avoid duplicates
	seenFighterIDs := make(map[string]bool)

	// Keep fetching pages until we exceed the empty page threshold
	emptyPageCount := 0
	maxEmptyPages := 3 // More conservative for country-specific pages

	for page := 0; emptyPageCount < maxEmptyPages; page++ {
		pageURL := fmt.Sprintf("%s&page=%d", countryURL, page)
		log.Printf("Scraping %s fighter page %d", countryName, page)

		resp, err := s.client.Get(pageURL)
		if err != nil {
			log.Printf("Error fetching %s page %d: %v", countryName, page, err)
			emptyPageCount++
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("Error parsing HTML for %s page %d: %v", countryName, page, err)
			emptyPageCount++
			continue
		}

		// Find fighters on this page
		pageFighters := []*Fighter{}
		doc.Find("div.c-listing-athlete-flipcard").Each(func(i int, item *goquery.Selection) {
			fighter := &Fighter{
				Rankings:    []FighterRanking{}, // Initialize empty rankings slice
				Nationality: countryName,        // Set nationality based on the country we're scraping
			}
			name := strings.TrimSpace(item.Find("span.c-listing-athlete__name").First().Text())
			var nickname string
			nicknameElem := item.Find("span.c-listing-athlete__nickname .field__item")
			if nicknameElem.Length() > 0 {
				nickname = strings.TrimSpace(nicknameElem.Text())
			}
			weightClass := strings.TrimSpace(item.Find("span.c-listing-athlete__title .field__item").First().Text())
			record := strings.TrimSpace(item.Find("span.c-listing-athlete__record").First().Text())
			profileURL, exists := item.Find("a.e-button--black").First().Attr("href")

			if exists {
				if strings.HasPrefix(profileURL, "/") {
					profileURL = "https://www.ufc.com" + profileURL
				}

				// Don't skip "-0" URLs anymore since we're using ufc_id as unique constraint

				urlParts := strings.Split(profileURL, "/")
				if len(urlParts) > 0 {
					fighter.UFCID = urlParts[len(urlParts)-1]
				}
				fighter.UFCURL = profileURL
			}

			// Set basic fighter information - use the original name
			fighter.Name = name // Use the original name by default
			fighter.Nickname = nickname
			fighter.WeightClass = weightClass
			fighter.Record = record
			fighter.Status = "Unknown"
			fighter.Ranking = "Unranked"

			// Only proceed if fighter has a name and UFC ID
			if fighter.Name != "" && fighter.UFCID != "" {
				// Skip if we've seen this fighter ID already in this run
				if _, exists := seenFighterIDs[fighter.UFCID]; exists {
					return
				}

				// Mark this fighter ID as seen
				seenFighterIDs[fighter.UFCID] = true
				pageFighters = append(pageFighters, fighter)
			}
		})

		// If we didn't find any fighters on this page, increment empty page counter
		if len(pageFighters) == 0 {
			emptyPageCount++
		} else {
			// Reset empty page counter if we found fighters
			emptyPageCount = 0
			fighters = append(fighters, pageFighters...)
		}
	}

	log.Printf("Found %d fighters from %s", len(fighters), countryName)
	return fighters, nil
}

// ScrapeFightersByAllCountries gets fighters country by country
func (s *FighterScraper) ScrapeFightersByAllCountries(ctx context.Context) ([]*Fighter, error) {
	var allFighters []*Fighter

	// First get all available countries and their codes
	countries, err := s.GetAvailableCountries()
	if err != nil {
		return nil, fmt.Errorf("error getting country list: %v", err)
	}

	log.Printf("Starting to scrape fighters from %d countries", len(countries))

	// Process each country
	for _, country := range countries {
		select {
		case <-ctx.Done():
			return allFighters, ctx.Err()
		default:
		}

		fighters, err := s.ScrapeFightersByCountry(country.Code)
		if err != nil {
			log.Printf("Warning: Error scraping %s fighters: %v", country.Name, err)
			continue
		}

		allFighters = append(allFighters, fighters...)

		// Be nice to the server with a short delay between countries
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("Completed scraping a total of %d fighters from all countries", len(allFighters))
	return allFighters, nil
}

func (s *FighterScraper) ScrapeFightersDirectly() ([]*Fighter, error) {
	var allFighters []*Fighter

	// Track unique fighter IDs to avoid duplicates
	seenFighterIDs := make(map[string]bool)

	// Keep fetching pages until we exceed the empty page threshold
	emptyPageCount := 0
	maxEmptyPages := 10

	for page := 0; emptyPageCount < maxEmptyPages; page++ {
		pageURL := fmt.Sprintf("%s?page=%d", s.ufcAthleteListURL, page)
		log.Printf("Scraping fighter page %d directly", page)

		resp, err := s.client.Get(pageURL)
		if err != nil {
			log.Printf("Error fetching page %d: %v", page, err)
			emptyPageCount++
			continue
		}

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("Error parsing HTML for page %d: %v", page, err)
			emptyPageCount++
			continue
		}

		// Find fighters on this page
		pageFighters := []*Fighter{}
		doc.Find("div.c-listing-athlete-flipcard").Each(func(i int, item *goquery.Selection) {
			fighter := &Fighter{
				Rankings: []FighterRanking{}, // Initialize empty rankings slice
			}

			name := strings.TrimSpace(item.Find("span.c-listing-athlete__name").First().Text())

			var nickname string
			nicknameElem := item.Find("span.c-listing-athlete__nickname .field__item")
			if nicknameElem.Length() > 0 {
				nickname = strings.TrimSpace(nicknameElem.Text())
			}

			weightClass := strings.TrimSpace(item.Find("span.c-listing-athlete__title .field__item").First().Text())
			record := strings.TrimSpace(item.Find("span.c-listing-athlete__record").First().Text())

			profileURL, exists := item.Find("a.e-button--black").First().Attr("href")
			if exists {
				if strings.HasPrefix(profileURL, "/") {
					profileURL = "https://www.ufc.com" + profileURL
				}

				// Don't skip "-0" URLs anymore since we're using ufc_id as unique constraint

				urlParts := strings.Split(profileURL, "/")
				if len(urlParts) > 0 {
					fighter.UFCID = urlParts[len(urlParts)-1]
				}
				fighter.UFCURL = profileURL
			}

			fighter.Name = name
			fighter.Nickname = nickname
			fighter.WeightClass = weightClass
			fighter.Record = record
			fighter.Status = "Unknown"
			fighter.Ranking = "Unranked"

			// Only add if fighter has a name and hasn't been seen before
			if fighter.Name != "" && fighter.UFCID != "" {
				if _, exists := seenFighterIDs[fighter.UFCID]; !exists {
					seenFighterIDs[fighter.UFCID] = true
					pageFighters = append(pageFighters, fighter)
				}
			}
		})

		// If we didn't find any fighters on this page, increment empty page counter
		if len(pageFighters) == 0 {
			emptyPageCount++
		} else {
			// Reset empty page counter if we found fighters
			emptyPageCount = 0
			allFighters = append(allFighters, pageFighters...)
		}
	}

	log.Printf("Found %d fighters directly from main listing", len(allFighters))
	return allFighters, nil
}

// GetFighterDetails fills in additional details for a fighter
func (s *FighterScraper) GetFighterDetails(fighter *Fighter) error {
	if fighter.UFCURL == "" {
		return fmt.Errorf("no URL provided for fighter %s", fighter.Name)
	}

	resp, err := s.client.Get(fighter.UFCURL)
	if err != nil {
		return fmt.Errorf("error fetching fighter page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("error parsing HTML: %v", err)
	}

	// Check for championship status in the hero profile
	isChampion := false
	isInterimChampion := false

	doc.Find("p.hero-profile__tag").Each(func(i int, tag *goquery.Selection) {
		text := strings.TrimSpace(tag.Text())

		// Check for championship status
		if text == "Title Holder" {
			isChampion = true
		}

		if strings.Contains(text, "Interim Title Holder") {
			isInterimChampion = true
		}
	})

	// Check for nickname - only update if there's actually a nickname element
	nicknameElem := doc.Find("p.hero-profile__nickname")
	if nicknameElem.Length() > 0 {
		fighter.Nickname = strings.TrimSpace(nicknameElem.Text())
	} else if fighter.Nickname != "" && strings.Contains(fighter.Nickname, "weight") {
		// If there's no nickname element but current nickname looks like a weight class, clear it
		fighter.Nickname = ""
	}

	// Update weight class if needed
	if fighter.WeightClass == "" {
		weightClass := strings.TrimSpace(doc.Find("p.hero-profile__division-title").First().Text())
		weightClass = strings.Replace(weightClass, "Division", "", -1)
		fighter.WeightClass = strings.TrimSpace(weightClass)
	}

	// Update record if needed
	if fighter.Record == "" {
		fighter.Record = strings.TrimSpace(doc.Find("p.hero-profile__division-body").First().Text())
	}

	// Extract bio info from the tabs section
	doc.Find(".c-bio__field").Each(func(i int, field *goquery.Selection) {
		label := strings.TrimSpace(field.Find(".c-bio__label").Text())
		text := strings.TrimSpace(field.Find(".c-bio__text").Text())

		switch label {
		case "Status":
			// Use the dedicated Status field from the bio section
			fighter.Status = text

			// Set ranking based on status
			if text == "Retired" || text == "Not Fighting" {
				fighter.Ranking = "Unranked"          // Retired/Not Fighting fighters are always unranked
				fighter.Rankings = []FighterRanking{} // Clear any existing rankings
			} else if text == "Active" {
				// Only set championship status if the fighter is active
				if isInterimChampion {
					fighter.Ranking = "Interim Champion"

					// Add to rankings slice too
					primaryWeightClass := strings.TrimSpace(fighter.WeightClass)
					if primaryWeightClass != "" {
						fighter.Rankings = append(fighter.Rankings, FighterRanking{
							WeightClass: primaryWeightClass,
							Rank:        "Interim Champion",
						})
					}
				} else if isChampion {
					fighter.Ranking = "Champion"

					// Add to rankings slice too
					primaryWeightClass := strings.TrimSpace(fighter.WeightClass)
					if primaryWeightClass != "" {
						fighter.Rankings = append(fighter.Rankings, FighterRanking{
							WeightClass: primaryWeightClass,
							Rank:        "Champion",
						})
					}
				}
			}

		case "Age":
			age, err := strconv.Atoi(text)
			if err == nil {
				fighter.Age = age
			}
		case "Height":
			fighter.Height = text
		case "Weight":
			fighter.Weight = text
		case "Reach":
			fighter.Reach = text
		}
	})

	// Extract win methods from the stats-records section
	doc.Find("div.stats-records--three-column").Each(func(i int, statsDiv *goquery.Selection) {
		// Find the title to ensure we're in the right section
		title := strings.TrimSpace(statsDiv.Find(".c-stat-3bar__title").Text())

		if title == "Win by Method" {
			// Extract win methods
			statsDiv.Find(".c-stat-3bar__group").Each(func(j int, group *goquery.Selection) {
				label := strings.TrimSpace(group.Find(".c-stat-3bar__label").Text())
				valueText := strings.TrimSpace(group.Find(".c-stat-3bar__value").Text())

				// Extract just the number from the value (e.g., from "7 (88%)")
				valueParts := strings.Fields(valueText)
				if len(valueParts) > 0 {
					valueStr := valueParts[0]
					value, err := strconv.Atoi(valueStr)
					if err == nil {
						switch label {
						case "KO/TKO":
							fighter.KOWins = value
						case "SUB":
							fighter.SubWins = value
						case "DEC":
							fighter.DecWins = value
						}
					}
				}
			})
		}
	})

	return nil
}

// ScrapeRankings gets the current UFC rankings
func (s *FighterScraper) ScrapeRankings() (map[string]map[string]string, error) {
	// Maps weight class -> (fighter ID -> ranking)
	rankings := make(map[string]map[string]string)

	resp, err := s.client.Get(s.ufcRankingsURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching rankings page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	doc.Find("div.view-grouping").Each(func(i int, groupingDiv *goquery.Selection) {
		// Extract the weight class name
		weightClassHeader := strings.TrimSpace(groupingDiv.Find("div.view-grouping-header").Text())

		// Skip pound-for-pound rankings
		if strings.Contains(weightClassHeader, "Pound-for-Pound") {
			return
		}

		// Create map for this weight class
		rankingsByFighterID := make(map[string]string)

		// Extract champion
		championLinkElem := groupingDiv.Find("div.rankings--athlete--champion a")
		if championLinkElem.Length() > 0 {
			championLink, exists := championLinkElem.Attr("href")
			if exists {
				urlParts := strings.Split(championLink, "/")
				if len(urlParts) > 0 {
					fighterID := urlParts[len(urlParts)-1]
					rankingsByFighterID[fighterID] = "Champion"
				}
			}
		}

		// Extract ranked fighters
		groupingDiv.Find("tbody tr").Each(func(j int, row *goquery.Selection) {
			rankText := strings.TrimSpace(row.Find("td.views-field-weight-class-rank").Text())
			fighterLink, exists := row.Find("td.views-field-title a").Attr("href")

			if exists && rankText != "" {
				urlParts := strings.Split(fighterLink, "/")
				if len(urlParts) > 0 {
					fighterID := urlParts[len(urlParts)-1]
					rankingsByFighterID[fighterID] = "#" + rankText
				}
			}
		})

		// Store the rankings for this weight class
		rankings[weightClassHeader] = rankingsByFighterID
	})

	return rankings, nil
}

// ScrapeAllFighters - main entry point that uses both country-based and direct approaches
func (s *FighterScraper) ScrapeAllFighters(ctx context.Context) ([]*Fighter, error) {
	log.Println("Starting to scrape UFC fighters using combined approach...")

	// First, get the total count of athletes from the UI
	expectedTotal, err := s.GetTotalAthleteCount()
	if err != nil {
		log.Printf("Warning: Couldn't determine total athlete count: %v", err)
		// Use a default high number if we can't get the actual count
		expectedTotal = 6000
	}

	// Step 1: Get all fighters by scraping each country's page
	fightersByCountry, err := s.ScrapeFightersByAllCountries(ctx)
	if err != nil {
		return nil, fmt.Errorf("error scraping fighters by nationality: %v", err)
	}

	log.Printf("Found %d fighters with nationality information", len(fightersByCountry))

	// Check if we've found all the fighters
	if len(fightersByCountry) >= expectedTotal {
		log.Printf("Found all expected fighters with nationality information (%d/%d)",
			len(fightersByCountry), expectedTotal)
		return s.processAndEnrichFighters(ctx, fightersByCountry)
	}

	// Step 2: If we didn't get all fighters, also scrape the main listing
	log.Printf("Need to supplement with direct scraping (%d/%d found so far)",
		len(fightersByCountry), expectedTotal)

	// Create a map of already seen fighter IDs
	seenFighterIDs := make(map[string]bool)
	for _, fighter := range fightersByCountry {
		if fighter.UFCID != "" {
			seenFighterIDs[fighter.UFCID] = true
		}
	}

	// Get remaining fighters directly
	directFighters, err := s.ScrapeFightersDirectly()
	if err != nil {
		log.Printf("Warning: Error scraping fighters directly: %v", err)
		// Continue with what we have
	} else {
		// Add only new fighters not already found via country scraping
		for _, fighter := range directFighters {
			if fighter.UFCID != "" && !seenFighterIDs[fighter.UFCID] {
				seenFighterIDs[fighter.UFCID] = true
				fightersByCountry = append(fightersByCountry, fighter)
			}
		}
	}

	log.Printf("Found a total of %d unique fighters after combining approaches", len(fightersByCountry))

	return s.processAndEnrichFighters(ctx, fightersByCountry)
}

// Update the processAndEnrichFighters function to simplify the duplicate handling now that we're using UFC ID
func (s *FighterScraper) processAndEnrichFighters(ctx context.Context, fighters []*Fighter) ([]*Fighter, error) {
	totalFighters := len(fighters)
	log.Printf("Retrieving detailed information for %d fighters...", totalFighters)

	// Build a map of UFC IDs to fighters to remove any duplicates
	fightersByID := make(map[string]*Fighter)
	for _, fighter := range fighters {
		if fighter.UFCID != "" {
			// If we have a duplicate, keep the one with nationality info
			if existing, exists := fightersByID[fighter.UFCID]; exists {
				if existing.Nationality == "" && fighter.Nationality != "" {
					fightersByID[fighter.UFCID] = fighter
				}
			} else {
				fightersByID[fighter.UFCID] = fighter
			}
		}
	}

	// Convert map back to slice
	uniqueFighters := make([]*Fighter, 0, len(fightersByID))
	for _, fighter := range fightersByID {
		uniqueFighters = append(uniqueFighters, fighter)
	}

	// Get details for each fighter
	for i, fighter := range uniqueFighters {
		select {
		case <-ctx.Done():
			return uniqueFighters, ctx.Err()
		default:
		}

		// Keep retrying until we get the details - never skip a fighter
		success := false
		attempts := 0
		maxAttempts := 3 // Limit retries to prevent infinite loops

		for !success && attempts < maxAttempts {
			err := s.GetFighterDetails(fighter)
			if err == nil {
				success = true
			} else {
				// Log the error but keep trying
				attempts++
				if attempts < maxAttempts {
					// Log retry message every few attempts to avoid log spam
					if attempts%2 == 0 {
						log.Printf("Still trying to get details for %s (attempt %d/%d): %v",
							fighter.Name, attempts, maxAttempts, err)
					}
					// Wait between retries - longer wait for more attempts
					backoffTime := time.Duration(50*attempts) * time.Millisecond
					if backoffTime > 1*time.Second {
						backoffTime = 1 * time.Second
					}
					time.Sleep(backoffTime)
				} else {
					// Log final failure but don't stop the process
					log.Printf("Warning: Couldn't get details for fighter %s after %d attempts: %v",
						fighter.Name, attempts, err)
				}
			}
		}

		if (i+1)%10 == 0 || i+1 == len(uniqueFighters) {
			log.Printf("Processed %d/%d fighters...", i+1, len(uniqueFighters))
		}
	}

	// Now that we have UFC_ID as the unique constraint, we no longer need to worry about duplicate names
	// We can keep all fighters and their original names

	// Get the rankings from the rankings page
	log.Println("Scraping fighter rankings from rankings page...")
	rankingsByWeightClass, err := s.ScrapeRankings()
	if err != nil {
		// Just log error and continue with what we have
		log.Printf("Warning: Error scraping rankings: %v", err)
	} else {
		// Apply rankings to fighters
		for _, fighter := range uniqueFighters {
			// Skip retired or not fighting fighters
			if fighter.Status == "Retired" || fighter.Status == "Not Fighting" {
				continue
			}

			// For each weight class in the rankings
			for weightClass, rankingsByFighterID := range rankingsByWeightClass {
				// If this fighter has a ranking in this weight class
				if ranking, exists := rankingsByFighterID[fighter.UFCID]; exists {
					// Initialize Rankings slice if it's nil
					if fighter.Rankings == nil {
						fighter.Rankings = []FighterRanking{}
					}

					// Add this ranking to the fighter's rankings
					newRanking := FighterRanking{
						WeightClass: weightClass,
						Rank:        ranking,
					}

					// Check if we already have a ranking for this weight class
					found := false
					for i, r := range fighter.Rankings {
						if r.WeightClass == weightClass {
							// Update existing ranking
							fighter.Rankings[i] = newRanking
							found = true
							break
						}
					}

					// If we don't have a ranking for this weight class yet, add it
					if !found {
						fighter.Rankings = append(fighter.Rankings, newRanking)
					}

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

	log.Printf("Completed scraping %d unique UFC fighters (including %d with nationality information)",
		len(uniqueFighters),
		countFightersWithNationality(uniqueFighters))

	return uniqueFighters, nil
}

// Helper to count fighters with nationality info
func countFightersWithNationality(fighters []*Fighter) int {
	count := 0
	for _, f := range fighters {
		if f.Nationality != "" {
			count++
		}
	}
	return count
}

func InsertFighter(db *sql.DB, fighter *Fighter) error {
	// Now that we're using ufc_id as the unique constraint, we can use a simple UPSERT pattern
	_, err := db.Exec(`
		INSERT INTO fighters
		(name, nickname, weight_class, record, status, ranking, ufc_id, ufc_url, nationality)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (ufc_id) DO UPDATE SET
			name = EXCLUDED.name,
			nickname = EXCLUDED.nickname,
			weight_class = EXCLUDED.weight_class, 
			record = EXCLUDED.record,
			status = EXCLUDED.status,
			ranking = EXCLUDED.ranking,
			ufc_url = EXCLUDED.ufc_url,
			nationality = EXCLUDED.nationality
	`, fighter.Name, fighter.Nickname, fighter.WeightClass, fighter.Record,
		fighter.Status, fighter.Ranking, fighter.UFCID, fighter.UFCURL, fighter.Nationality)

	return err
}

type UFCFighterProfileScraper struct {
	client *http.Client
}

// Create a new instance of the UFC fighter profile scraper
func NewUFCFighterProfileScraper() *UFCFighterProfileScraper {
	return &UFCFighterProfileScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ScrapeFighterHistory gets all fights for a specific fighter from their UFC profile
func (s *UFCFighterProfileScraper) ScrapeFighterHistory(ufcURL string) ([]UFCScrapedFight, error) {
	var fights []UFCScrapedFight

	resp, err := s.client.Get(ufcURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unsuccessful response: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract fighter name - adjust selector based on UFC website structure
	fighterName := strings.TrimSpace(doc.Find("h1.hero-profile__name").Text())
	if fighterName == "" {
		// Try alternate selector
		fighterName = strings.TrimSpace(doc.Find(".field--name-name").Text())
	}

	log.Printf("Scraping fight history for %s", fighterName)

	// Find the fight history section - adjust selectors based on UFC website structure
	doc.Find(".c-card-event--athlete-results").Each(func(i int, fightCard *goquery.Selection) {
		// Extract opponent info
		opponentName := strings.TrimSpace(fightCard.Find(".c-card-event--result__opponent-name").Text())
		weightClass := strings.TrimSpace(fightCard.Find(".c-card-event--result__weightclass").Text())

		// Only proceed if we have opponent and weight class
		if opponentName == "" || weightClass == "" {
			return
		}

		// Extract result
		resultText := strings.TrimSpace(fightCard.Find(".c-card-event--result__caption").Text())
		methodText := strings.TrimSpace(fightCard.Find(".c-card-event--result__method").Text())

		// Determine if it was a main event or title fight
		isMainEvent := fightCard.Find(".c-card-event--result__headline").HasClass("main-event") ||
			strings.Contains(strings.ToLower(resultText), "main event")

		isTitleFight := strings.Contains(strings.ToLower(weightClass), "title") ||
			strings.Contains(strings.ToLower(resultText), "title")

		// Determine winner and result
		var fighter1Result, fighter2Result string
		if strings.Contains(strings.ToLower(resultText), "win") {
			fighter1Result = "Win"
			fighter2Result = "Loss"
		} else if strings.Contains(strings.ToLower(resultText), "loss") {
			fighter1Result = "Loss"
			fighter2Result = "Win"
		} else if strings.Contains(strings.ToLower(resultText), "draw") {
			fighter1Result = "Draw"
			fighter2Result = "Draw"
		} else if strings.Contains(strings.ToLower(resultText), "nc") ||
			strings.Contains(strings.ToLower(resultText), "no contest") {
			fighter1Result = "NC"
			fighter2Result = "NC"
		}

		// Split fighter names into parts
		fighterNameParts := strings.Fields(fighterName)
		opponentNameParts := strings.Fields(opponentName)

		var fighter1GivenName, fighter1LastName, fighter2GivenName, fighter2LastName string

		if len(fighterNameParts) > 1 {
			fighter1GivenName = strings.Join(fighterNameParts[:len(fighterNameParts)-1], " ")
			fighter1LastName = fighterNameParts[len(fighterNameParts)-1]
		} else if len(fighterNameParts) == 1 {
			fighter1LastName = fighterNameParts[0]
		}

		if len(opponentNameParts) > 1 {
			fighter2GivenName = strings.Join(opponentNameParts[:len(opponentNameParts)-1], " ")
			fighter2LastName = opponentNameParts[len(opponentNameParts)-1]
		} else if len(opponentNameParts) == 1 {
			fighter2LastName = opponentNameParts[0]
		}

		// Extract round and time information if available
		roundInfo := strings.TrimSpace(fightCard.Find(".c-card-event--result__time").Text())

		var round, time string
		if roundInfo != "" {
			// Try to parse "Round X XX:XX" format
			parts := strings.Split(roundInfo, " ")
			if len(parts) >= 3 {
				round = parts[1] // Just the number
				time = parts[2]  // The time
			}
		}

		// Create fight object
		fight := UFCScrapedFight{
			Fighter1Name:      fighterName,
			Fighter1GivenName: fighter1GivenName,
			Fighter1LastName:  fighter1LastName,
			Fighter1Result:    fighter1Result,
			Fighter2Name:      opponentName,
			Fighter2GivenName: fighter2GivenName,
			Fighter2LastName:  fighter2LastName,
			Fighter2Result:    fighter2Result,
			WeightClass:       weightClass,
			Method:            methodText,
			Round:             round,
			Time:              time,
			IsMainEvent:       isMainEvent,
			IsTitleFight:      isTitleFight,
		}

		// Add the fight to our list
		fights = append(fights, fight)
	})

	log.Printf("Found %d fights for %s", len(fights), fighterName)
	return fights, nil
}

func BackfillFighterHistory(db *sql.DB, fighterName string) error {
	log.Printf("Backfilling fight history for %s", fighterName)

	// First, find the fighter ID
	var fighterID string
	err := db.QueryRow("SELECT id FROM fighters WHERE LOWER(name) LIKE '%' || LOWER($1) || '%'", fighterName).Scan(&fighterID)
	if err != nil {
		return fmt.Errorf("fighter not found: %v", err)
	}

	// Get events that should have this fighter's bouts (from Wikipedia URLs or other sources)
	// For Volkanovski and Lopes, we can hardcode known events
	var eventURLs []string

	if strings.Contains(strings.ToLower(fighterName), "volkanovski") {
		eventURLs = []string{
			"https://www.ufc.com/event/ufc-284",                          // vs Makhachev
			"https://www.ufc.com/event/ufc-276",                          // vs Holloway 3
			"https://www.ufc.com/event/ufc-273",                          // vs Korean Zombie
			"https://www.ufc.com/event/ufc-266",                          // vs Ortega
			"https://www.ufc.com/event/ufc-251",                          // vs Holloway 2
			"https://www.ufc.com/event/ufc-245",                          // vs Holloway 1
			"https://www.ufc.com/event/ufc-237",                          // vs Aldo
			"https://www.ufc.com/event/ufc-232",                          // vs Mendes
			"https://www.ufc.com/event/ufc-fight-night-november-18-2018", // vs Elkins
		}
	} else if strings.Contains(strings.ToLower(fighterName), "lopes") {
		eventURLs = []string{
			"https://www.ufc.com/event/ufc-288",                       // vs Evloev
			"https://www.ufc.com/event/ufc-fight-night-april-22-2023", // vs Jourdain
		}
	}

	// Create UFC scraper
	ufcScraper := NewUFCFightScraper()

	// Process each event
	for _, eventURL := range eventURLs {
		log.Printf("Scraping event: %s for fighter %s", eventURL, fighterName)

		// Get the event ID from our database
		var eventID string
		eventName := extractEventNameFromURL(eventURL)
		err := db.QueryRow("SELECT id FROM events WHERE name LIKE '%' || $1 || '%'", eventName).Scan(&eventID)
		if err != nil {
			log.Printf("Couldn't find event ID for %s: %v", eventName, err)
			continue
		}

		// Scrape fights from this event
		fights, err := ufcScraper.ScrapeFights(eventURL)
		if err != nil {
			log.Printf("Error scraping event %s: %v", eventURL, err)
			continue
		}

		log.Printf("Found %d fights at event %s", len(fights), eventName)

		// Look for fights involving our fighter
		for _, fight := range fights {
			if strings.Contains(strings.ToLower(fight.Fighter1Name), strings.ToLower(fighterName)) ||
				strings.Contains(strings.ToLower(fight.Fighter2Name), strings.ToLower(fighterName)) {

				log.Printf("Found fight: %s vs %s", fight.Fighter1Name, fight.Fighter2Name)

				// Find opponent's ID
				var opponentName string
				if strings.Contains(strings.ToLower(fight.Fighter1Name), strings.ToLower(fighterName)) {
					opponentName = fight.Fighter2Name
				} else {
					opponentName = fight.Fighter1Name
				}

				var opponentID string
				err := db.QueryRow("SELECT id FROM fighters WHERE LOWER(name) LIKE '%' || LOWER($1) || '%'", opponentName).Scan(&opponentID)
				if err != nil {
					log.Printf("Opponent %s not found: %v", opponentName, err)
					continue
				}

				// Determine fighter1_id and fighter2_id
				var fighter1ID, fighter2ID string
				var fighter1Name, fighter2Name string

				if strings.Contains(strings.ToLower(fight.Fighter1Name), strings.ToLower(fighterName)) {
					fighter1ID = fighterID
					fighter2ID = opponentID
					fighter1Name = fight.Fighter1Name
					fighter2Name = fight.Fighter2Name
				} else {
					fighter1ID = opponentID
					fighter2ID = fighterID
					fighter1Name = fight.Fighter1Name
					fighter2Name = fight.Fighter2Name
				}

				// Determine winner
				var winnerID *string
				if (strings.Contains(strings.ToLower(fight.Fighter1Name), strings.ToLower(fighterName)) &&
					fight.Fighter1Result == "Win") ||
					(strings.Contains(strings.ToLower(fight.Fighter2Name), strings.ToLower(fighterName)) &&
						fight.Fighter2Result == "Win") {
					winnerID = &fighterID
				} else if (strings.Contains(strings.ToLower(fight.Fighter1Name), strings.ToLower(fighterName)) &&
					fight.Fighter1Result == "Loss") ||
					(strings.Contains(strings.ToLower(fight.Fighter2Name), strings.ToLower(fighterName)) &&
						fight.Fighter2Result == "Loss") {
					winnerID = &opponentID
				}

				// Insert or update the fight
				_, err = db.Exec(`
                    INSERT INTO fights (
                        event_id, fighter1_id, fighter2_id, fighter1_name, fighter2_name, 
                        weight_class, is_main_event, was_title_fight, winner_id,
                        fighter1_rank, fighter2_rank,
                        created_at, updated_at
                    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
                    ON CONFLICT (event_id, fighter1_id, fighter2_id) DO UPDATE SET
                        fighter1_name = EXCLUDED.fighter1_name,
                        fighter2_name = EXCLUDED.fighter2_name,
                        weight_class = EXCLUDED.weight_class,
                        is_main_event = EXCLUDED.is_main_event,
                        was_title_fight = EXCLUDED.was_title_fight,
                        winner_id = EXCLUDED.winner_id,
                        fighter1_rank = EXCLUDED.fighter1_rank,
                        fighter2_rank = EXCLUDED.fighter2_rank,
                        updated_at = EXCLUDED.updated_at
                `,
					eventID, fighter1ID, fighter2ID, fighter1Name, fighter2Name,
					fight.WeightClass, fight.IsMainEvent, fight.IsTitleFight, winnerID,
					fight.Fighter1Rank, fight.Fighter2Rank,
					time.Now(), time.Now(),
				)

				if err != nil {
					log.Printf("Failed to save fight %s vs %s: %v", fight.Fighter1Name, fight.Fighter2Name, err)
				} else {
					log.Printf("Successfully saved fight: %s vs %s at event %s",
						fight.Fighter1Name, fight.Fighter2Name, eventName)
				}
			}
		}
	}

	return nil
}

func extractEventNameFromURL(url string) string {
	// Parse out the event name from URLs like:
	// https://www.ufc.com/event/ufc-284
	// https://www.ufc.com/event/ufc-fight-night-april-22-2023

	parts := strings.Split(url, "/")
	if len(parts) < 1 {
		return ""
	}

	lastPart := parts[len(parts)-1]

	// Convert to more readable format
	lastPart = strings.ReplaceAll(lastPart, "-", " ")

	// Handle numbered events
	if strings.HasPrefix(lastPart, "ufc ") && len(lastPart) > 4 {
		if num, err := strconv.Atoi(lastPart[4:]); err == nil {
			return fmt.Sprintf("UFC %d", num)
		}
	}

	// Handle Fight Nights
	if strings.Contains(lastPart, "fight night") {
		return "UFC Fight Night"
	}

	return strings.ToUpper(lastPart)
}
