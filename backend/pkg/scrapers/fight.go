package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tebeka/selenium"
)

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

type ChromeDriverConfig struct {
	Path       string
	Port       int
	WaitTimeS  int
	Headless   bool
	UserAgent  string
	EnableLogs bool
}

type UFCFightScraper struct {
	// Configuration
	config         *ScraperConfig
	chromeConfig   *ChromeDriverConfig
	useSelenium    bool
	chromedriverWD selenium.WebDriver
	service        *selenium.Service

	// Traditional scraper elements (fallback only)
	client      *http.Client
	clientPool  []*http.Client
	userAgent   string
	poolMutex   sync.Mutex
	rateLimiter <-chan time.Time
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func NewUFCFightScraper(config *ScraperConfig) *UFCFightScraper {
	// Create a pool of HTTP clients for fallback method
	const clientPoolSize = 4
	clientPool := make([]*http.Client, clientPoolSize)

	for i := 0; i < clientPoolSize; i++ {
		clientPool[i] = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
			},
		}
	}

	// Rate limit requests to 3 per second to avoid triggering anti-scraping measures
	rateLimiter := time.Tick(3 * time.Second)

	// Set user agent
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	if config != nil && config.UserAgent != "" {
		userAgent = config.UserAgent
	}

	// Create default ChromeDriver config
	chromeConfig := &ChromeDriverConfig{
		Path:       getEnvOrDefault("CHROME_DRIVER_PATH", "C:\\Users\\AppData\\Local\\etc"),
		Port:       4444,
		WaitTimeS:  5,
		Headless:   true,
		UserAgent:  userAgent,
		EnableLogs: false,
	}

	// Create the scraper
	scraper := &UFCFightScraper{
		config:       config,
		chromeConfig: chromeConfig,
		useSelenium:  false,
		client:       clientPool[0],
		clientPool:   clientPool,
		userAgent:    userAgent,
		rateLimiter:  rateLimiter,
	}

	// Try to set up Selenium automatically, but don't fail if it doesn't work
	// This allows the scraper to still work with HTTP as a fallback
	err := scraper.SetupSelenium(nil)
	if err != nil {
		log.Printf("Warning: Failed to set up Selenium automatically: %v", err)
		log.Printf("Falling back to HTTP client for scraping (may not work well with dynamic content)")
	}

	return scraper
}

func (s *UFCFightScraper) SetupSelenium(chromeConfig *ChromeDriverConfig) error {
	if chromeConfig != nil {
		s.chromeConfig = chromeConfig
	}

	// Check if chromedriver exists
	if s.chromeConfig.Path != "chromedriver" {
		if _, err := os.Stat(s.chromeConfig.Path); os.IsNotExist(err) {
			return fmt.Errorf("ChromeDriver not found at: %s", s.chromeConfig.Path)
		}
	}

	// Clean up any existing sessions
	if s.service != nil {
		s.service.Stop()
	}
	if s.chromedriverWD != nil {
		s.chromedriverWD.Quit()
	}

	// Set up ChromeDriver capabilities with Chrome options
	caps := selenium.Capabilities{
		"browserName": "chrome",
	}

	// Add Chrome-specific options
	chromeOpts := map[string]interface{}{
		"args": []string{},
	}

	// Add headless flag if enabled
	if s.chromeConfig.Headless {
		chromeOpts["args"] = append(chromeOpts["args"].([]string),
			"--headless",
			"--no-sandbox",
			"--disable-dev-shm-usage",
		)
	}

	// Set user agent
	if s.chromeConfig.UserAgent != "" {
		chromeOpts["args"] = append(chromeOpts["args"].([]string),
			fmt.Sprintf("--user-agent=%s", s.chromeConfig.UserAgent),
		)
	}

	// Add SSL error handling options
	chromeOpts["args"] = append(chromeOpts["args"].([]string),
		"--ignore-certificate-errors",
		"--ignore-ssl-errors",
		"--disable-web-security",
	)

	// Add Chrome options to capabilities
	caps["goog:chromeOptions"] = chromeOpts

	// Start a ChromeDriver service with a retry mechanism
	var err error
	maxServiceRetries := 3

	for retry := 0; retry < maxServiceRetries; retry++ {
		if retry > 0 {
			log.Printf("Retrying ChromeDriver service start (attempt %d/%d)", retry+1, maxServiceRetries)
			time.Sleep(2 * time.Second) // Wait before retry
		}

		// If port is in use, try incrementing it
		adjustedPort := s.chromeConfig.Port + retry
		s.service, err = selenium.NewChromeDriverService(s.chromeConfig.Path, adjustedPort)
		if err == nil {
			// Update port for WebDriver connection
			s.chromeConfig.Port = adjustedPort
			break
		}
	}

	if err != nil {
		return fmt.Errorf("error starting ChromeDriver service after %d attempts: %w", maxServiceRetries, err)
	}

	// Connect to the WebDriver instance with retry
	maxConnectRetries := 3

	for retry := 0; retry < maxConnectRetries; retry++ {
		if retry > 0 {
			log.Printf("Retrying WebDriver connection (attempt %d/%d)", retry+1, maxConnectRetries)
			time.Sleep(2 * time.Second) // Wait before retry
		}

		s.chromedriverWD, err = selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", s.chromeConfig.Port))
		if err == nil {
			break
		}
	}

	if err != nil {
		s.service.Stop()
		return fmt.Errorf("error connecting to ChromeDriver after %d attempts: %w", maxConnectRetries, err)
	}

	// Set page load timeout
	if err := s.chromedriverWD.SetPageLoadTimeout(30 * time.Second); err != nil {
		log.Printf("Warning: Failed to set page load timeout: %v", err)
	}

	// Removed the SetScriptTimeout line since it's not available in your version of selenium

	s.useSelenium = true
	return nil
}

func (s *UFCFightScraper) IsUsingSelenium() bool {
	return s.useSelenium
}

func (s *UFCFightScraper) getClient() *http.Client {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	// Round-robin selection from client pool
	client := s.clientPool[0]
	s.clientPool = append(s.clientPool[1:], s.clientPool[0])
	return client
}

func (s *UFCFightScraper) ScrapeFights(ufcURL string) ([]UFCScrapedFight, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second) // Increased timeout for Selenium
	defer cancel()

	return s.ScrapeFightsWithContext(ctx, ufcURL)
}

func (s *UFCFightScraper) ScrapeFightsWithContext(ctx context.Context, ufcURL string) ([]UFCScrapedFight, error) {
	var doc *goquery.Document
	var err error

	if s.useSelenium {
		// Always use Selenium if it's available
		doc, err = s.fetchWithSelenium(ctx, ufcURL)
		if err != nil {
			log.Printf("Selenium fetch failed: %v - trying to fallback to HTTP client", err)
			// Fall back to HTTP client if Selenium fails
			doc, err = s.fetchWithHTTPClient(ctx, ufcURL)
		}
	} else {
		// Fall back to regular HTTP client if Selenium is not set up
		doc, err = s.fetchWithHTTPClient(ctx, ufcURL)
	}

	if err != nil {
		return nil, err
	}

	// Find all fight elements
	fightElements := doc.Find(".c-listing-fight")

	// Pre-allocate fights slice to the expected size
	fights := make([]UFCScrapedFight, 0, fightElements.Length())

	// Process fights sequentially to maintain order
	fightElements.Each(func(i int, fightElement *goquery.Selection) {
		fight := s.extractFightInfo(i, fightElement)

		if fight != nil && fight.Fighter1Name != "" && fight.Fighter2Name != "" {
			name1 := strings.ReplaceAll(fight.Fighter1Name, "vs", "")
			name2 := strings.ReplaceAll(fight.Fighter2Name, "vs", "")

			if len(strings.TrimSpace(name1)) > 0 && len(strings.TrimSpace(name2)) > 0 {
				// The first fight (i=0) is the main event
				fight.IsMainEvent = (i == 0)
				fights = append(fights, *fight)
			}
		}
	})

	// Deduplicate fights while maintaining order
	uniqueFights := s.deduplicateFights(fights)

	return uniqueFights, nil
}

func (s *UFCFightScraper) fetchWithSelenium(ctx context.Context, ufcURL string) (*goquery.Document, error) {
    // Check if context is already canceled
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        // Continue if context is still valid
    }

    // Add recovery mechanism
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Recovered from panic in Selenium fetch: %v", r)
            // Try to restart the browser session
            if s.chromedriverWD != nil {
                s.chromedriverWD.Quit()
            }
            
            // Restart the session in a separate function call
            _ = s.SetupSelenium(nil)
        }
    }()

    // Add retry mechanism for navigation
    var err error
    maxNavigationRetries := 3
    
    for retry := 0; retry < maxNavigationRetries; retry++ {
        if retry > 0 {
            log.Printf("Retrying navigation to URL (attempt %d/%d)", retry+1, maxNavigationRetries)
            time.Sleep(2 * time.Second) // Wait before retry
        }
        
        // Navigate to the URL
        err = s.chromedriverWD.Get(ufcURL)
        if err == nil {
            break // Success
        }
    }
    
    if err != nil {
        return nil, fmt.Errorf("error navigating to URL after %d attempts: %w", maxNavigationRetries, err)
    }

    // Log that we're waiting
    log.Printf("Page loaded with Selenium, waiting %d seconds for JavaScript execution...", s.chromeConfig.WaitTimeS)
    
    // Wait for the specified time to let JavaScript execute
    // This is a critical step - we need to wait long enough for the dynamic content to load
    time.Sleep(time.Duration(s.chromeConfig.WaitTimeS) * time.Second)
    
    // Get the page source after JavaScript has executed
    log.Println("Selenium wait complete, getting page source...")
    
    // Add retry for page source retrieval
    var htmlContent string
    maxSourceRetries := 3
    
    for retry := 0; retry < maxSourceRetries; retry++ {
        if retry > 0 {
            log.Printf("Retrying page source retrieval (attempt %d/%d)", retry+1, maxSourceRetries)
            time.Sleep(2 * time.Second) // Wait before retry
        }
        
        htmlContent, err = s.chromedriverWD.PageSource()
        if err == nil && len(htmlContent) > 0 {
            break // Success
        }
    }
    
    if err != nil {
        return nil, fmt.Errorf("error getting page source after %d attempts: %w", maxSourceRetries, err)
    }

    if len(htmlContent) == 0 {
        return nil, fmt.Errorf("failed to retrieve HTML content from page")
    }

    // Save the HTML content to a file for debugging (optional)
    if s.chromeConfig.EnableLogs {
        if err := os.WriteFile("selenium_page_source.html", []byte(htmlContent), 0644); err != nil {
            log.Printf("Warning: Failed to save debug HTML: %v", err)
        } else {
            log.Println("Debug HTML saved to selenium_page_source.html")
        }
    }

    // Parse the HTML with goquery
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
    if err != nil {
        return nil, fmt.Errorf("failed to parse HTML: %w", err)
    }

    return doc, nil
}

func (s *UFCFightScraper) fetchWithHTTPClient(ctx context.Context, ufcURL string) (*goquery.Document, error) {
	// Respect rate limiting
	select {
	case <-s.rateLimiter:
		// Continue after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	log.Println("Using HTTP client as fallback (dynamic content may not load correctly)...")

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", ufcURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent to mimic a browser
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	// Get client from pool
	client := s.getClient()

	// Make HTTP request
	resp, err := client.Do(req)
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

	return doc, nil
}

func (s *UFCFightScraper) deduplicateFights(fights []UFCScrapedFight) []UFCScrapedFight {
    seen := make(map[string]bool)
    uniqueFights := make([]UFCScrapedFight, 0, len(fights))
    
    for _, fight := range fights {
        // Create a unique key for each fight
        key := fmt.Sprintf("%s-%s-%s",
            strings.ToLower(fight.Fighter1Name),
            strings.ToLower(fight.Fighter2Name),
            strings.ToLower(fight.WeightClass))
            
        // Only add if we haven't seen this fight before
        if !seen[key] {
            // Fix duplicated text issues (except for time)
            fight.Method = fixDuplicatedText(fight.Method)
            fight.Round = fixDuplicatedText(fight.Round)
            // Leave Time as is - no fixDuplicatedText
            
            uniqueFights = append(uniqueFights, fight)
            seen[key] = true
        }
    }
    
    return uniqueFights
}

func (s *UFCFightScraper) extractFightInfo(index int, fightElement *goquery.Selection) *UFCScrapedFight {
	// Initialize the fight
	fight := &UFCScrapedFight{}

	// Weight class
	weightClass := fightElement.Find(".c-listing-fight__class-text").First().Text()
	fight.WeightClass = cleanText(weightClass)

	// Check if title fight
	fight.IsTitleFight = strings.Contains(strings.ToLower(weightClass), "title")

	// Main event is typically the first fight
	fight.IsMainEvent = index == 0

	// Find corner names
	cornerNames := fightElement.Find(".c-listing-fight__corner-name")

	// Process first fighter
	if cornerNames.Length() > 0 {
		redCorner := cornerNames.Eq(0)
		givenName := cleanText(redCorner.Find(".c-listing-fight__corner-given-name").Text())
		familyName := cleanText(redCorner.Find(".c-listing-fight__corner-family-name").Text())

		// Fallback if given/family name extraction fails
		if givenName == "" || familyName == "" {
			fullName := cleanText(redCorner.Text())
			nameParts := strings.Fields(fullName)
			if len(nameParts) > 0 {
				givenName = nameParts[0]
				familyName = strings.Join(nameParts[1:], " ")
			}
		}

		fight.Fighter1GivenName = givenName
		fight.Fighter1LastName = familyName
		fight.Fighter1Name = strings.TrimSpace(givenName + " " + familyName)
	}

	// Process second fighter
	if cornerNames.Length() > 1 {
		blueCorner := cornerNames.Eq(1)
		givenName := cleanText(blueCorner.Find(".c-listing-fight__corner-given-name").Text())
		familyName := cleanText(blueCorner.Find(".c-listing-fight__corner-family-name").Text())

		// Fallback if given/family name extraction fails
		if givenName == "" || familyName == "" {
			fullName := cleanText(blueCorner.Text())
			nameParts := strings.Fields(fullName)
			if len(nameParts) > 0 {
				givenName = nameParts[0]
				familyName = strings.Join(nameParts[1:], " ")
			}
		}

		fight.Fighter2GivenName = givenName
		fight.Fighter2LastName = familyName
		fight.Fighter2Name = strings.TrimSpace(givenName + " " + familyName)
	}

	// Check for special result outcomes first (Draw, No Contest)
	redDrawElement := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome--draw")
	blueDrawElement := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome--draw")
	redNoContestElement := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome--no-contest")
	blueNoContestElement := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome--no-contest")

	// Check for Draw
	if (redDrawElement.Length() > 0 && strings.Contains(redDrawElement.Text(), "Draw")) ||
		(blueDrawElement.Length() > 0 && strings.Contains(blueDrawElement.Text(), "Draw")) {
		fight.Fighter1Result = "Draw"
		fight.Fighter2Result = "Draw"
		// Set method to Draw as well to indicate the fight result
		fight.Method = "Draw"
	} else if (redNoContestElement.Length() > 0) || (blueNoContestElement.Length() > 0) {
		// Check for No Contest
		fight.Fighter1Result = "No Contest"
		fight.Fighter2Result = "No Contest"
		// Set method to No Contest as well
		fight.Method = "No Contest"
	} else {
		// Extract results for red corner (fighter 1) if not a special result
		redWinElement := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome--win")
		if redWinElement.Length() > 0 {
			fight.Fighter1Result = "Win"
		} else {
			redOutcome := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome").Text()
			if redOutcome != "" {
				fight.Fighter1Result = cleanText(redOutcome)
			}
		}

		// Extract results for blue corner (fighter 2) if not a special result
		blueWinElement := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome--win")
		if blueWinElement.Length() > 0 {
			fight.Fighter2Result = "Win"
		} else {
			blueOutcome := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome").Text()
			if blueOutcome != "" {
				fight.Fighter2Result = cleanText(blueOutcome)
			}
		}

		// If we still don't have results, try to infer from the method
		if fight.Fighter1Result == "" && fight.Fighter2Result == "" {
			method := fightElement.Find(".c-listing-fight__result-text.method").Text()

			// Check if the method text indicates a draw or no contest
			methodLower := strings.ToLower(method)
			if strings.Contains(methodLower, "draw") {
				fight.Fighter1Result = "Draw"
				fight.Fighter2Result = "Draw"
				fight.Method = "Draw"
			} else if strings.Contains(methodLower, "no contest") || strings.Contains(methodLower, "nc") {
				fight.Fighter1Result = "No Contest"
				fight.Fighter2Result = "No Contest"
				fight.Method = "No Contest"
			} else if method != "" {
				// Look for special case - banner or award that might indicate winner
				redBanner := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__banner")
				blueBanner := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__banner")

				if redBanner.Length() > 0 && strings.Contains(strings.ToLower(redBanner.Text()), "win") {
					fight.Fighter1Result = "Win"
				} else if blueBanner.Length() > 0 && strings.Contains(strings.ToLower(blueBanner.Text()), "win") {
					fight.Fighter2Result = "Win"
				}
			}
		}
	}

	// Get fighter ranks
	ranks := fightElement.Find(".c-listing-fight__corner-rank")
	if ranks.Length() >= 2 {
		// First rank is for red corner, second for blue corner
		fight.Fighter1Rank = cleanText(ranks.Eq(0).Text())
		fight.Fighter2Rank = cleanText(ranks.Eq(1).Text())
	} else {
		// Try alternate rank selectors if the primary ones don't work
		redRank := fightElement.Find(".js-listing-fight__corner-rank:first-child span").Text()
		blueRank := fightElement.Find(".js-listing-fight__corner-rank:last-child span").Text()

		if redRank != "" {
			fight.Fighter1Rank = cleanText(redRank)
		}
		if blueRank != "" {
			fight.Fighter2Rank = cleanText(blueRank)
		}
	}

	// Fight results
	roundText := fightElement.Find(".c-listing-fight__result-text.round").Text()
	timeText := fightElement.Find(".c-listing-fight__result-text.time").Text()
	methodText := fightElement.Find(".c-listing-fight__result-text.method").Text()

	// Fix duplicated text for round and method but leave time as-is
	fight.Round = fixDuplicatedText(cleanText(roundText))
	fight.Time = cleanText(timeText) // No fixDuplicatedText for time - preserve original format
	
	// Only set Method if not already set as a special result
	if fight.Method == "" {
		fight.Method = fixDuplicatedText(cleanText(methodText))
	}

	return fight
}

func (s *UFCFightScraper) markMainEvent(fights []UFCScrapedFight) {
	// Reset all main event flags
	for i := range fights {
		fights[i].IsMainEvent = false
	}

	// First look for title fights - priority to heavyweight title
	var titleFightIndex int = -1
	var heavyweightTitleIndex int = -1

	for i, fight := range fights {
		weightClass := strings.ToLower(fight.WeightClass)
		if strings.Contains(weightClass, "title") {
			if titleFightIndex == -1 {
				titleFightIndex = i
			}

			if strings.Contains(weightClass, "heavyweight") {
				heavyweightTitleIndex = i
				break // Heavyweight title is definitely the main event
			}
		}
	}

	// Set main event based on priority
	if heavyweightTitleIndex >= 0 {
		fights[heavyweightTitleIndex].IsMainEvent = true
	} else if titleFightIndex >= 0 {
		fights[titleFightIndex].IsMainEvent = true
	} else if len(fights) > 0 {
		// If no title bout found, look for fights with higher-ranked fighters
		bestRankSum := 1000 // Lower is better for ranks
		bestRankIndex := 0

		for i, fight := range fights {
			// Extract ranks (assuming format like "#1", "#C", etc.)
			rank1 := extractRankNumber(fight.Fighter1Rank)
			rank2 := extractRankNumber(fight.Fighter2Rank)
			rankSum := rank1 + rank2

			if rankSum < bestRankSum {
				bestRankSum = rankSum
				bestRankIndex = i
			}
		}

		fights[bestRankIndex].IsMainEvent = true
	}
}

func extractRankNumber(rank string) int {
	rank = strings.TrimSpace(strings.ToLower(rank))

	if rank == "" {
		return 100 // Unranked
	}

	// Check for champion
	if strings.Contains(rank, "c") {
		return 0
	}

	// Extract numeric part
	re := regexp.MustCompile(`#?(\d+)`)
	matches := re.FindStringSubmatch(rank)
	if len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}

	return 100 // Default to unranked
}

func fixDuplicatedText(text string) string {
    if len(text) == 0 {
        return text
    }
    
    // Special handling for time values (expected format like "5:00" or "1:23")
    timeRegex := regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
    if timeRegex.MatchString(text) {
        return text // Return time values as-is
    }
    
    // Handle duplicated time formats with leading zeros (like "00:05:00" should be "5:00")
    timeWithLeadingZerosRegex := regexp.MustCompile(`^0*(\d{1,2}):(\d{2})(?::0{2})?$`)
    if matches := timeWithLeadingZerosRegex.FindStringSubmatch(text); len(matches) > 2 {
        // Return in the format "M:SS"
        minutes, _ := strconv.Atoi(matches[1])
        return fmt.Sprintf("%d:%s", minutes, matches[2])
    }
    
    // Check for exact duplication (same text repeated)
    halfLen := len(text) / 2
    if halfLen > 0 && text[:halfLen] == text[halfLen:] {
        return text[:halfLen]
    }
    
    // Try a more sophisticated approach for pattern detection
    for i := 1; i <= len(text)/2; i++ {
        if i <= 0 {
            continue
        }
        
        pattern := text[:i]
        repeats := true
        
        for j := i; j < len(text); j += i {
            end := j + i
            if end > len(text) {
                end = len(text)
            }
            
            if text[j:end] != pattern[:end-j] {
                repeats = false
                break
            }
        }
        
        if repeats {
            return pattern
        }
    }
    
    return text
}

func cleanText(text string) string {
	// Remove newlines and tabs
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Replace multiple spaces with a single space
	text = strings.Join(strings.Fields(text), " ")

	return strings.TrimSpace(text)
}

func (s *UFCFightScraper) ScrapeMultipleEvents(urls []string) (map[string][]UFCScrapedFight, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // Extended timeout for sequential processing
	defer cancel()

	// Create a map to store results
	results := make(map[string][]UFCScrapedFight)
	
	// Track already processed URLs to prevent duplicates
	processedURLs := make(map[string]bool)
	
	// Process URLs sequentially, one at a time
	for _, url := range urls {
		// Check if URL already processed
		if processedURLs[url] {
			log.Printf("Skipping already processed URL: %s", url)
			continue
		}
		
		// Mark URL as processed
		processedURLs[url] = true
		
		log.Printf("Processing URL: %s", url)
		
		// Scrape fights for this event
		fights, err := s.ScrapeFightsWithContext(ctx, url)
		if err != nil {
			log.Printf("Error scraping %s: %v", url, err)
			continue // Continue with next URL instead of stopping entirely
		}
		
		// Store results
		results[url] = fights
		
		// Add a delay between requests to avoid overloading the server
		time.Sleep(3 * time.Second)
	}

	return results, nil
}

func (s *UFCFightScraper) Close() {
	if s.useSelenium {
		// Catch any panics during close
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic during scraper close: %v", r)
			}
		}()

		if s.chromedriverWD != nil {
			// Use a timeout context for quitting
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			quitChan := make(chan error, 1)

			go func() {
				quitChan <- s.chromedriverWD.Quit()
			}()

			select {
			case err := <-quitChan:
				if err != nil {
					log.Printf("Error during WebDriver quit: %v", err)
				}
			case <-ctx.Done():
				log.Printf("WebDriver quit timed out: %v", ctx.Err())
			}

			s.chromedriverWD = nil
		}

		if s.service != nil {
			s.service.Stop()
			s.service = nil
		}

		// Mark Selenium as no longer in use
		s.useSelenium = false
	}
}

func (s *UFCFightScraper) ScrapeFightsWithHTTP(ctx context.Context, ufcURL string) ([]UFCScrapedFight, error) {
	// Save current state
	originalUseSelenium := s.useSelenium

	// Force HTTP client
	s.useSelenium = false

	// Restore original state when done
	defer func() {
		s.useSelenium = originalUseSelenium
	}()

	// Use the existing context-based scrape method
	return s.ScrapeFightsWithContext(ctx, ufcURL)
}

func (s *UFCFightScraper) ScrapeFightsWithSelenium(ctx context.Context, ufcURL string) ([]UFCScrapedFight, error) {
    // If Selenium isn't set up yet, try to set it up
    if !s.useSelenium {
        log.Printf("Setting up Selenium for event scraping")
        err := s.SetupSelenium(nil)
        if err != nil {
            return nil, fmt.Errorf("failed to set up Selenium: %w", err)
        }
    }

    // Force use of Selenium regardless of the scraper's default settings
    originalUseSelenium := s.useSelenium
    s.useSelenium = true
    
    // Make sure to restore the original setting when done
    defer func() {
        s.useSelenium = originalUseSelenium
    }()
    
    log.Printf("Fetching page with Selenium to ensure dynamic content loads...")
    
    // Force to use fetchWithSelenium instead of going through ScrapeFightsWithContext
    doc, err := s.fetchWithSelenium(ctx, ufcURL)
    if err != nil {
        return nil, fmt.Errorf("selenium fetch failed: %w", err)
    }
    
    // Find all fight elements
    fightElements := doc.Find(".c-listing-fight")
    
    // Pre-allocate fights slice to the expected size
    fights := make([]UFCScrapedFight, 0, fightElements.Length())
    
    log.Printf("Found %d fight elements on the page", fightElements.Length())
    
    // Process fights sequentially to maintain order
    fightElements.Each(func(i int, fightElement *goquery.Selection) {
        fight := s.extractFightInfo(i, fightElement)
        
        if fight != nil && fight.Fighter1Name != "" && fight.Fighter2Name != "" {
            name1 := strings.ReplaceAll(fight.Fighter1Name, "vs", "")
            name2 := strings.ReplaceAll(fight.Fighter2Name, "vs", "")
            
            if len(strings.TrimSpace(name1)) > 0 && len(strings.TrimSpace(name2)) > 0 {
                // The first fight (i=0) is the main event
                fight.IsMainEvent = (i == 0)
                fights = append(fights, *fight)
            }
        }
    })
    
    // Deduplicate fights while maintaining order
    uniqueFights := s.deduplicateFights(fights)
    
    log.Printf("Extracted %d unique fights with Selenium", len(uniqueFights))
    // Debug output to verify fight results are being captured
    for i, fight := range uniqueFights {
        log.Printf("Fight %d: %s vs %s - Results: %s vs %s", 
            i+1, fight.Fighter1Name, fight.Fighter2Name, fight.Fighter1Result, fight.Fighter2Result)
    }
    
    return uniqueFights, nil
}