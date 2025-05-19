package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

	Wins    int
	KOWins  int
	SubWins int
	DecWins int
	Draws   int
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

// Optimized version that processes countries in parallel
func (s *FighterScraper) ScrapeFightersByAllCountries(ctx context.Context) ([]*Fighter, error) {
	var allFighters []*Fighter
	var mu sync.Mutex // Protect allFighters

	// First get all available countries and their codes
	countries, err := s.GetAvailableCountries()
	if err != nil {
		return nil, fmt.Errorf("error getting country list: %v", err)
	}

	log.Printf("Starting to scrape fighters from %d countries in parallel", len(countries))

	// Determine optimal number of workers
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Cap at a reasonable number to avoid overwhelming the UFC server
	}

	// Create a channel to distribute countries
	countryCh := make(chan CountryCode, len(countries))

	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup

	// Launch worker goroutines
	var processedCountries, totalFighters int32

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for country := range countryCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				fighters, err := s.ScrapeFightersByCountry(country.Code)
				if err != nil {
					log.Printf("Worker %d: Error scraping %s fighters: %v",
						workerID, country.Name, err)
					continue
				}

				// Add fighters to the global list
				if len(fighters) > 0 {
					mu.Lock()
					allFighters = append(allFighters, fighters...)
					mu.Unlock()

					atomic.AddInt32(&totalFighters, int32(len(fighters)))
				}

				count := atomic.AddInt32(&processedCountries, 1)
				log.Printf("Progress: %d/%d countries processed, %d fighters found so far",
					count, len(countries), atomic.LoadInt32(&totalFighters))

				// Be nice to the server with a short delay between countries
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	// Send countries to the workers
	for _, country := range countries {
		countryCh <- country
	}
	close(countryCh)

	// Wait for all workers to finish
	wg.Wait()

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

		// Add a small delay between pages to avoid overwhelming the server
		time.Sleep(200 * time.Millisecond)
	}

	log.Printf("Found %d fighters directly from main listing", len(allFighters))
	return allFighters, nil
}

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

	// Parse W-L-D from record
	var totalWins int
	recordText := strings.TrimSpace(doc.Find("p.hero-profile__division-body").First().Text())

	if recordText != "" {
		// Update fighter record if it's empty
		if fighter.Record == "" {
			fighter.Record = recordText
		}

		// Log the record text for debugging
		log.Printf("Fighter %s record: %s", fighter.Name, recordText)

		// Parse wins, losses, and draws from the record
		// Example format: "23-5-0 (W-L-D)" or "21-1-1"
		recordRegex := regexp.MustCompile(`^(\d+)-(\d+)-(\d+)`)
		matches := recordRegex.FindStringSubmatch(recordText)
		if len(matches) >= 4 {
			totalWins, _ = strconv.Atoi(matches[1])
			fighter.Wins = totalWins        // Store the total wins in the Wins field
			_, _ = strconv.Atoi(matches[2]) // Ignore losses with blank identifier
			draws, _ := strconv.Atoi(matches[3])

			// Set the draws in the fighter struct
			fighter.Draws = draws
			log.Printf("Parsed record for %s: wins=%d, draws=%d", fighter.Name, totalWins, draws)
		} else {
			log.Printf("Failed to parse record for %s: %s", fighter.Name, recordText)
		}
	}

	// Default status to "Active" if status is Unknown
	if fighter.Status == "Unknown" {
		fighter.Status = "Active"
	}

	// Extract bio info from the tabs section
	statusFound := false
	doc.Find(".c-bio__field").Each(func(i int, field *goquery.Selection) {
		label := strings.TrimSpace(field.Find(".c-bio__label").Text())
		text := strings.TrimSpace(field.Find(".c-bio__text").Text())

		switch label {
		case "Status":
			statusFound = true
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

	// If we didn't find the status field, confirm that we set it to Active
	if !statusFound && fighter.Status == "Unknown" {
		fighter.Status = "Active"
	}

	// Reset win methods
	fighter.KOWins = 0
	fighter.SubWins = 0
	fighter.DecWins = 0

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

	// Sanity check win methods
	sumOfWinMethods := fighter.KOWins + fighter.SubWins + fighter.DecWins
	if totalWins > 0 && sumOfWinMethods != totalWins {
		// Reset win methods if they don't match the total wins
		log.Printf("Win methods don't match for %s: totalWins=%d, sumOfMethods=%d (KO=%d, SUB=%d, DEC=%d)",
			fighter.Name, totalWins, sumOfWinMethods, fighter.KOWins, fighter.SubWins, fighter.DecWins)
		fighter.KOWins = 0
		fighter.SubWins = 0
		fighter.DecWins = 0
	}

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

// Optimized version for parallel processing of fighter details
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

	// Create worker pool for parallel processing
	numWorkers := runtime.NumCPU()
	if numWorkers > 10 {
		numWorkers = 10 // Cap at a reasonable maximum
	}

	fighterCh := make(chan *Fighter, len(uniqueFighters))
	var wg sync.WaitGroup

	// Stats counters
	var successCount, failureCount int32

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for fighter := range fighterCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Attempt to get fighter details with retries
				success := false
				attempts := 0
				maxAttempts := 3

				for !success && attempts < maxAttempts {
					err := s.GetFighterDetails(fighter)
					if err == nil {
						success = true
						atomic.AddInt32(&successCount, 1)
					} else {
						attempts++
						if attempts < maxAttempts {
							// Exponential backoff
							backoffTime := time.Duration(100*(1<<attempts)) * time.Millisecond
							if backoffTime > 1*time.Second {
								backoffTime = 1 * time.Second
							}
							time.Sleep(backoffTime)
						} else {
							atomic.AddInt32(&failureCount, 1)
							log.Printf("Worker %d: Failed to get details for %s after %d attempts: %v",
								workerID, fighter.Name, attempts, err)
						}
					}
				}

				// Log progress periodically
				totalProcessed := atomic.LoadInt32(&successCount) + atomic.LoadInt32(&failureCount)
				if totalProcessed%50 == 0 || totalProcessed == int32(len(uniqueFighters)) {
					log.Printf("Fighter details: %d/%d processed (%d successful, %d failed)",
						totalProcessed, len(uniqueFighters), successCount, failureCount)
				}
			}
		}(i)
	}

	// Send fighters to workers
	for _, fighter := range uniqueFighters {
		fighterCh <- fighter
	}
	close(fighterCh)

	// Wait for all workers to finish
	wg.Wait()

	log.Printf("Completed getting fighter details: %d successful, %d failed",
		successCount, failureCount)

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

	log.Printf("Completed processing %d unique UFC fighters (including %d with nationality information)",
		len(uniqueFighters),
		countFightersWithNationality(uniqueFighters))

	return uniqueFighters, nil
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
	// Parse the wins, losses, draws from the struct fields
	wins := fighter.Wins

	// Now that we're using ufc_id as the unique constraint, we can use a simple UPSERT pattern
	_, err := db.Exec(`
		INSERT INTO fighters
		(name, nickname, weight_class, status, rank, ufc_id, ufc_url, nationality,
		 wins, ko_wins, sub_wins, dec_wins, draws, age, height, weight, reach)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (ufc_id) DO UPDATE SET
			name = EXCLUDED.name,
			nickname = EXCLUDED.nickname,
			weight_class = EXCLUDED.weight_class,
			status = EXCLUDED.status,
			rank = EXCLUDED.rank,
			ufc_url = EXCLUDED.ufc_url,
			nationality = EXCLUDED.nationality,
			wins = EXCLUDED.wins,
			ko_wins = EXCLUDED.ko_wins,
			sub_wins = EXCLUDED.sub_wins,
			dec_wins = EXCLUDED.dec_wins,
			draws = EXCLUDED.draws,
			age = EXCLUDED.age,
			height = EXCLUDED.height,
			weight = EXCLUDED.weight,
			reach = EXCLUDED.reach
	`, fighter.Name, fighter.Nickname, fighter.WeightClass,
		fighter.Status, fighter.Ranking, fighter.UFCID, fighter.UFCURL, fighter.Nationality,
		wins, fighter.KOWins, fighter.SubWins, fighter.DecWins, fighter.Draws,
		fighter.Age, fighter.Height, fighter.Weight, fighter.Reach)

	return err
}
