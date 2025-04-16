package scrapers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"regexp"
    "strconv"
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
	client      *http.Client
	clientPool  []*http.Client
	userAgent   string
	poolMutex   sync.Mutex
	rateLimiter <-chan time.Time
}

// NewUFCFightScraper creates a new UFCFightScraper
func NewUFCFightScraper(config *ScraperConfig) *UFCFightScraper {
	// Create a pool of HTTP clients for better performance
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
	rateLimiter := time.Tick(333 * time.Millisecond)

	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
	if config != nil && config.UserAgent != "" {
		userAgent = config.UserAgent
	}

	return &UFCFightScraper{
		client:      clientPool[0],
		clientPool:  clientPool,
		userAgent:   userAgent,
		rateLimiter: rateLimiter,
	}
}

// getClient returns an HTTP client from the pool
func (s *UFCFightScraper) getClient() *http.Client {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	// Round-robin selection from client pool
	client := s.clientPool[0]
	s.clientPool = append(s.clientPool[1:], s.clientPool[0])
	return client
}

// ScrapeFights retrieves all fights from a UFC event page
func (s *UFCFightScraper) ScrapeFights(ufcURL string) ([]UFCScrapedFight, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return s.ScrapeFightsWithContext(ctx, ufcURL)
}

func (s *UFCFightScraper) ScrapeFightsWithContext(ctx context.Context, ufcURL string) ([]UFCScrapedFight, error) {
	// Respect rate limiting
	select {
	case <-s.rateLimiter:
		// Continue after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

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
            // Fix duplicated text issues
            fight.Method = fixDuplicatedText(fight.Method)
            fight.Round = fixDuplicatedText(fight.Round)
            fight.Time = fixDuplicatedText(fight.Time)
            
            uniqueFights = append(uniqueFights, fight)
            seen[key] = true
        }
    }
    
    return uniqueFights
}

// markMainEvent identifies and marks the main event in a list of fights
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

// extractRankNumber converts rank strings like "#1", "#C", "C" to numeric values
// Champion = 0, #1 = 1, #2 = 2, etc., unranked = 100
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

// fixDuplicatedText fixes text that has been duplicated, like "SUBSUB" -> "SUB"
func fixDuplicatedText(text string) string {
	if len(text) == 0 {
		return text
	}
	
	// Check for exact duplication (same text repeated)
	halfLen := len(text) / 2
	if halfLen > 0 && text[:halfLen] == text[halfLen:] {
		return text[:halfLen]
	}
	
	return text
}

// extractFightInfo pulls fight information from a single fight element
func (s *UFCFightScraper) extractFightInfo(index int, fightElement *goquery.Selection) *UFCScrapedFight {
	// Initialize the fight pointer
	fight := &UFCScrapedFight{}

	// Weight class
	weightClass := fightElement.Find(".c-listing-fight__class-text").First().Text()
	fight.WeightClass = cleanText(weightClass)

	// Check if title fight
	fight.IsTitleFight = strings.Contains(strings.ToLower(weightClass), "title")

	// Main event is typically the first fight
	fight.IsMainEvent = index == 0

	// Find corner names more flexibly
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

	// Extract results
	resultContainers := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome-wrapper, .c-listing-fight__corner-body--blue .c-listing-fight__outcome-wrapper")
	if resultContainers.Length() >= 2 {
		fight.Fighter1Result = cleanText(resultContainers.Eq(0).Text())
		fight.Fighter2Result = cleanText(resultContainers.Eq(1).Text())
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

	return fight
}

// extractFighterInfo extracts information for a single fighter
func extractFighterInfo(
	fightElement *goquery.Selection,
	nameSelector string,
	resultSelector string,
	fullName *string,
	givenName *string,
	lastName *string,
	result *string,
) {
	cornerName := fightElement.Find(nameSelector)

	// Check if the fighter name is split into given/family name parts
	givenNameText := cornerName.Find(".c-listing-fight__corner-given-name").Text()
	familyNameText := cornerName.Find(".c-listing-fight__corner-family-name").Text()

	if givenNameText != "" && familyNameText != "" {
		*givenName = cleanText(givenNameText)
		*lastName = cleanText(familyNameText)
		*fullName = *givenName + " " + *lastName
	} else {
		// If not split, get the full name
		*fullName = cleanText(cornerName.Text())

		// Try to split the name if possible
		nameParts := strings.Fields(*fullName)
		if len(nameParts) >= 2 {
			*givenName = strings.Join(nameParts[:len(nameParts)-1], " ")
			*lastName = nameParts[len(nameParts)-1]
		}
	}

	// Get fight outcome
	*result = cleanText(fightElement.Find(resultSelector).Text())
}

// cleanText removes extra whitespace and normalizes text
func cleanText(text string) string {
	// Remove newlines and tabs
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Replace multiple spaces with a single space using a more efficient approach
	text = strings.Join(strings.Fields(text), " ")

	return strings.TrimSpace(text)
}

// ScrapeMultipleEvents concurrently scrapes fights from multiple UFC event URLs
func (s *UFCFightScraper) ScrapeMultipleEvents(urls []string) (map[string][]UFCScrapedFight, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create a map to store results
	results := make(map[string][]UFCScrapedFight)
	var resultsMutex sync.Mutex

	// Use error group to track errors
	var wg sync.WaitGroup
	errorChan := make(chan error, len(urls))

	// Process up to 3 events concurrently
	sem := make(chan struct{}, 3)

	for _, url := range urls {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(url string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			// Scrape fights for this event
			fights, err := s.ScrapeFightsWithContext(ctx, url)
			if err != nil {
				errorChan <- fmt.Errorf("error scraping %s: %w", url, err)
				return
			}

			// Store results
			resultsMutex.Lock()
			results[url] = fights
			resultsMutex.Unlock()
		}(url)
	}

	// Wait for all scraping to complete
	wg.Wait()
	close(errorChan)

	// Check if any errors occurred
	for err := range errorChan {
		// Return the first error
		return results, err
	}

	return results, nil
}
