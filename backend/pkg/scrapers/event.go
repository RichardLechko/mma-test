package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"mma-scheduler/internal/models"
)

type UFCEventScraper struct {
	client      *http.Client
	clientPool  []*http.Client
	baseURL     string
	userAgent   string
	rateLimiter <-chan time.Time
	poolMutex   sync.Mutex
}

type EventDetail struct {
	URL      string
	Name     string
	Event    *models.Event
	Error    error
	PageNum  int
}

// NewUFCEventScraper creates a new scraper for UFC events
func NewUFCEventScraper(config *ScraperConfig) *UFCEventScraper {
	// Create a pool of HTTP clients to distribute load
	const clientPoolSize = 5
	clientPool := make([]*http.Client, clientPoolSize)
	
	for i := 0; i < clientPoolSize; i++ {
		clientPool[i] = &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
			},
		}
	}

	// Create rate limiter to avoid overwhelming the server
	rateLimiter := time.Tick(250 * time.Millisecond) // 4 requests per second
	
	// Set default user agent if not provided
	userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36"
	if config != nil && config.UserAgent != "" {
		userAgent = config.UserAgent
	}

	return &UFCEventScraper{
		client:      clientPool[0],
		clientPool:  clientPool,
		baseURL:     "https://www.ufc.com/events",
		userAgent:   userAgent,
		rateLimiter: rateLimiter,
	}
}

// getClient returns an HTTP client from the pool
func (s *UFCEventScraper) getClient() *http.Client {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	// Round-robin selection from client pool
	client := s.clientPool[0]
	s.clientPool = append(s.clientPool[1:], s.clientPool[0])
	return client
}

// ScrapeEvents scrapes events from UFC.com, focusing on UFC URL, event date, venue, city, and country
func (s *UFCEventScraper) ScrapeEvents(ctx context.Context) ([]*models.Event, error) {
	var allEvents []*models.Event
	processedEvents := make(map[string]bool)
	var mu sync.Mutex
	
	// Process each page sequentially
	page := 0
	consecutiveEmptyPages := 0
	maxConsecutiveEmpty := 5
	
	for consecutiveEmptyPages < maxConsecutiveEmpty {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return allEvents, ctx.Err()
		default:
			// Continue processing
		}
		
		// Construct URL with pagination
		pageURL := s.baseURL
		if page > 0 {
			pageURL = fmt.Sprintf("%s?page=%d", s.baseURL, page)
		}
		
		// Process page
		log.Printf("Processing page %d", page)
		events, err := s.processPage(ctx, pageURL)
		if err != nil {
			log.Printf("Error processing page %d: %v", page, err)
			consecutiveEmptyPages++
			page++
			continue
		}
		
		// Check if we found any events
		if len(events) == 0 {
			consecutiveEmptyPages++
			log.Printf("Page %d empty, consecutive empty pages: %d/%d", 
				page, consecutiveEmptyPages, maxConsecutiveEmpty)
		} else {
			consecutiveEmptyPages = 0
			log.Printf("Page %d found %d events", page, len(events))
			
			// Add events to our collection
			mu.Lock()
			for _, event := range events {
				if !processedEvents[event.UFCURL] {
					allEvents = append(allEvents, event)
					processedEvents[event.UFCURL] = true
					log.Printf("Found event: %s on %s at %s",
						event.Name,
						event.Date.Format("2006-01-02"),
						event.Venue)
				}
			}
			mu.Unlock()
		}
		
		page++
		
		// Add delay between pages
		time.Sleep(250 * time.Millisecond)
	}
	
	log.Printf("Scraping completed. Found %d events", len(allEvents))
	return allEvents, nil
}

// processPage processes a single page and returns all events found
func (s *UFCEventScraper) processPage(ctx context.Context, pageURL string) ([]*models.Event, error) {
	// Respect rate limiting
	select {
	case <-s.rateLimiter:
		// Continue after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	
	// Get client from pool
	client := s.getClient()
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	// Set headers
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	
	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}
	
	var events []*models.Event
	
	// Find all event cards
	doc.Find(".c-card-event--result").Each(func(i int, card *goquery.Selection) {
		// Find event link
		linkElem := card.Find(".c-card-event--result__headline a")
		eventURL, exists := linkElem.Attr("href")
		if !exists || eventURL == "" {
			return
		}
		
		// Ensure full URL
		if !strings.HasPrefix(eventURL, "http") {
			eventURL = "https://www.ufc.com" + eventURL
		}
		
		// Extract event name
		eventName := strings.TrimSpace(linkElem.Text())
		
		// Create event with the URL
		event := &models.Event{
			UFCURL: eventURL,
			Name:   eventName,
		}
		
		// Extract date information
		dateElem := card.Find(".c-card-event--result__date")
		dateText := strings.TrimSpace(dateElem.Text())
		
		// Try to get timestamp from data attribute
		timestampStr, exists := dateElem.Attr("data-main-card-timestamp")
		if exists {
			// Convert Unix timestamp to time.Time
			timestampInt := 0
			fmt.Sscanf(timestampStr, "%d", &timestampInt)
			if timestampInt > 0 {
				// Use the original timestamp with correct date and time
				event.Date = time.Unix(int64(timestampInt), 0)
			}
		}
		
		// If timestamp failed, try to parse the text date
		if event.Date.IsZero() {
			// Clean up the date text before parsing
			dateText = strings.Split(dateText, " / ")[0] // Take only the date part
			dateFormats := []string{
				"Mon, Jan 2",
				"Monday, January 2",
			}
			
			for _, format := range dateFormats {
				parsed, err := time.Parse(format, dateText)
				if err == nil {
					// Use current year for the date since we couldn't get exact year
					currentYear := time.Now().Year()
					event.Date = time.Date(
						currentYear, 
						parsed.Month(), 
						parsed.Day(), 
						0, 0, 0, 0, 
						time.UTC,
					)
					
					// If the resulting date is in the past by more than a month, 
					// assume it's for next year
					if event.Date.Before(time.Now().AddDate(0, -1, 0)) {
						event.Date = time.Date(
							currentYear + 1, 
							parsed.Month(), 
							parsed.Day(), 
							0, 0, 0, 0, 
							time.UTC,
						)
					}
					break
				}
			}
		}
		
		// Extract location information
		locationElem := card.Find(".c-card-event--result__location")
		
		// Extract venue
		venueElem := locationElem.Find(".field--name-taxonomy-term-title h5")
		if venueElem.Length() > 0 {
			event.Venue = strings.TrimSpace(venueElem.Text())
		}
		
		// Extract city and country
		cityElem := locationElem.Find(".field--name-location .locality")
		if cityElem.Length() > 0 {
			event.City = strings.TrimSpace(cityElem.Text())
		}
		
		countryElem := locationElem.Find(".field--name-location .country")
		if countryElem.Length() > 0 {
			event.Country = strings.TrimSpace(countryElem.Text())
		}
		
		// Determine status based on date
		if !event.Date.IsZero() && event.Date.Before(time.Now()) {
			event.Status = "Completed"
		} else {
			event.Status = "Upcoming"
		}
		
		// If we have incomplete basic info, fetch the details
		if event.Date.IsZero() || event.Venue == "" || event.City == "" || event.Country == "" {
			detailEvent, err := s.scrapeEventDetails(ctx, eventURL)
			if err == nil && detailEvent != nil {
				// Merge detail data
				if event.Date.IsZero() && !detailEvent.Date.IsZero() {
					event.Date = detailEvent.Date
				}
				if event.Venue == "" && detailEvent.Venue != "" {
					event.Venue = detailEvent.Venue
				}
				if event.City == "" && detailEvent.City != "" {
					event.City = detailEvent.City
				}
				if event.Country == "" && detailEvent.Country != "" {
					event.Country = detailEvent.Country
				}
			}
		}
		
		// Add event if we have sufficient information
		if event.UFCURL != "" && (event.Venue != "" || event.City != "" || 
			event.Country != "" || !event.Date.IsZero()) {
			events = append(events, event)
		}
	})
	
	return events, nil
}

// scrapePage scrapes a single page of events
func (s *UFCEventScraper) scrapePage(
	ctx context.Context, 
	page int, 
	detailSem chan struct{},
	detailWg *sync.WaitGroup,
	eventDetailChan chan<- EventDetail,
) int {
	// Respect rate limiting
	select {
	case <-s.rateLimiter:
		// Continue after rate limit delay
	case <-ctx.Done():
		return 0
	}

	// Construct URL with pagination
	pageURL := s.baseURL
	if page > 0 {
		pageURL = fmt.Sprintf("%s?page=%d", s.baseURL, page)
	}

	// Get client from pool
	client := s.getClient()
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		log.Printf("Error creating request for page %d: %v", page, err)
		return 0
	}
	
	// Set headers
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	// Make request
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error fetching page %d: %v", page, err)
		return 0
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("Bad status code for page %d: %d", page, resp.StatusCode)
		return 0
	}
	
	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("Error parsing HTML for page %d: %v", page, err)
		return 0
	}

	// Track events found on this page
	eventsFound := 0
	
	// Process each event card
	doc.Find(".c-card-event--result").Each(func(i int, card *goquery.Selection) {
		// Find event link - this is our primary key (UFC URL)
		linkElem := card.Find(".c-card-event--result__headline a")
		eventURL, exists := linkElem.Attr("href")
		if !exists || eventURL == "" {
			return
		}

		// Ensure full URL
		if !strings.HasPrefix(eventURL, "http") {
			eventURL = "https://www.ufc.com" + eventURL
		}

		// Extract event name for logging
		eventName := strings.TrimSpace(linkElem.Text())
		
		// Extract basic event info
		event, basicInfoFound := s.extractBasicEventInfo(card, eventURL)
		
		// Need to scrape details?
		needDetails := !basicInfoFound
		
		// Queue up detail scraping in a worker
		if needDetails {
			detailWg.Add(1)
			go func(url, name string, baseEvent *models.Event) {
				// Use recover to prevent goroutine panics from crashing the app
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Recovered from panic in detail scraper for %s: %v", name, r)
					}
					detailWg.Done()
				}()
				
				// Use semaphore to limit concurrent detail requests
				select {
				case detailSem <- struct{}{}:
					// Successfully acquired semaphore
				case <-ctx.Done():
					// Context cancelled while waiting for semaphore
					return
				}
				
				// Setup deferred release of semaphore
				defer func() {
					select {
					case <-detailSem:
						// Successfully released
					default:
						// Channel might be full or closed, just continue
					}
				}()
				
				// Respect rate limiting
				select {
				case <-s.rateLimiter:
					// Continue after rate limit delay
				case <-ctx.Done():
					return
				}
				
				// Create timeout context for detail fetching
				detailCtx, detailCancel := context.WithTimeout(ctx, 30*time.Second)
				defer detailCancel()
				
				// Fetch details
				detailEvent, err := s.scrapeEventDetails(detailCtx, url)
				
				// Check if context is cancelled before sending results
				select {
				case <-ctx.Done():
					return
				default:
					// Continue with sending results
				}
				
				// Create the result to send
				var result EventDetail
				
				// Merge with base event if we have one
				if err == nil && detailEvent != nil && baseEvent != nil {
					if baseEvent.Date.IsZero() && !detailEvent.Date.IsZero() {
						baseEvent.Date = detailEvent.Date
					}
					if baseEvent.Venue == "" && detailEvent.Venue != "" {
						baseEvent.Venue = detailEvent.Venue
					}
					if baseEvent.City == "" && detailEvent.City != "" {
						baseEvent.City = detailEvent.City
					}
					if baseEvent.Country == "" && detailEvent.Country != "" {
						baseEvent.Country = detailEvent.Country
					}
					
					result = EventDetail{
						URL:     url,
						Name:    name,
						Event:   baseEvent,
						PageNum: page,
					}
				} else if err == nil && detailEvent != nil {
					// Use the detail event
					detailEvent.UFCURL = url
					result = EventDetail{
						URL:     url,
						Name:    name,
						Event:   detailEvent,
						PageNum: page,
					}
				} else {
					// Send error
					result = EventDetail{
						URL:     url,
						Name:    name,
						Error:   err,
						PageNum: page,
					}
				}
				
				// Send the result, but don't block indefinitely
				select {
				case eventDetailChan <- result:
					// Successfully sent
				case <-ctx.Done():
					// Context cancelled while sending
				}
			}(eventURL, eventName, event)
		} else if event != nil {
			// Send the event without details, but safely
			select {
			case eventDetailChan <- EventDetail{
				URL:     eventURL,
				Name:    eventName,
				Event:   event,
				PageNum: page,
			}:
				// Successfully sent
			case <-ctx.Done():
				// Context cancelled while sending
				return
			}
		}
		
		eventsFound++
	})

	return eventsFound
}

// extractBasicEventInfo extracts event information from the event card
func (s *UFCEventScraper) extractBasicEventInfo(card *goquery.Selection, eventURL string) (*models.Event, bool) {
	// Create event
	event := &models.Event{
		UFCURL: eventURL,
	}
	
	completeInfo := false
	
	// Extract date information
	dateElem := card.Find(".c-card-event--result__date")
	dateText := strings.TrimSpace(dateElem.Text())
	
	// Try to get timestamp from data attribute first
	timestampStr, exists := dateElem.Attr("data-main-card-timestamp")
	if exists {
		// Convert Unix timestamp to time.Time
		timestampInt := 0
		fmt.Sscanf(timestampStr, "%d", &timestampInt)
		if timestampInt > 0 {
			// Use the original timestamp with correct date and time
			event.Date = time.Unix(int64(timestampInt), 0)
		}
	}
	
	// If timestamp failed, try to parse the text date
	if event.Date.IsZero() {
		// Clean up the date text before parsing
		dateText = strings.Split(dateText, " / ")[0] // Take only the date part
		dateFormats := []string{
			"Mon, Jan 2",
			"Monday, January 2",
		}
		
		for _, format := range dateFormats {
			parsed, err := time.Parse(format, dateText)
			if err == nil {
				// Use current year for the date since we couldn't get exact year
				currentYear := time.Now().Year()
				event.Date = time.Date(
					currentYear, 
					parsed.Month(), 
					parsed.Day(), 
					0, 0, 0, 0, 
					time.UTC,
				)
				
				// If the resulting date is in the past by more than a month, 
				// assume it's for next year
				if event.Date.Before(time.Now().AddDate(0, -1, 0)) {
					event.Date = time.Date(
						currentYear + 1, 
						parsed.Month(), 
						parsed.Day(), 
						0, 0, 0, 0, 
						time.UTC,
					)
				}
				break
			}
		}
	}
	
	// Extract location information
	locationElem := card.Find(".c-card-event--result__location")
	
	// Extract venue
	venueElem := locationElem.Find(".field--name-taxonomy-term-title h5")
	if venueElem.Length() > 0 {
		event.Venue = strings.TrimSpace(venueElem.Text())
	}
	
	// Extract city and country
	cityElem := locationElem.Find(".field--name-location .locality")
	if cityElem.Length() > 0 {
		event.City = strings.TrimSpace(cityElem.Text())
	}
	
	countryElem := locationElem.Find(".field--name-location .country")
	if countryElem.Length() > 0 {
		event.Country = strings.TrimSpace(countryElem.Text())
	}
	
	// Determine if we have complete information
	if !event.Date.IsZero() && event.Venue != "" && event.City != "" && event.Country != "" {
		completeInfo = true
	}
	
	// Determine status based on date
	if !event.Date.IsZero() && event.Date.Before(time.Now()) {
		event.Status = "Completed"
	} else {
		event.Status = "Upcoming"
	}
	
	return event, completeInfo
}

// scrapeEventDetails fetches additional details from an event's page
func (s *UFCEventScraper) scrapeEventDetails(ctx context.Context, eventURL string) (*models.Event, error) {
	// Get client from pool
	client := s.getClient()
	
	req, err := http.NewRequestWithContext(ctx, "GET", eventURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching event page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}
	
	event := &models.Event{}
	
	// Extract date using data attributes which are more reliable
	dateContainer := doc.Find(".hero-event-results__date-container")
	if dateContainer.Length() > 0 {
		// Try to get timestamp from data attribute
		timestampStr, exists := dateContainer.Attr("data-main-card-timestamp")
		if exists {
			// Convert Unix timestamp to time.Time
			timestampInt := 0
			fmt.Sscanf(timestampStr, "%d", &timestampInt)
			if timestampInt > 0 {
				// Use the original timestamp with correct date and time
				event.Date = time.Unix(int64(timestampInt), 0)
			}
		}
		
		// If that fails, try to parse the text date
		if event.Date.IsZero() {
			dateText := strings.TrimSpace(dateContainer.Text())
			dateFormats := []string{
				"Mon, Jan 2",
				"Monday, January 2",
			}
			
			for _, format := range dateFormats {
				parsed, err := time.Parse(format, dateText)
				if err == nil {
					// Use current year for the date
					currentYear := time.Now().Year()
					event.Date = time.Date(
						currentYear, 
						parsed.Month(), 
						parsed.Day(), 
						0, 0, 0, 0, 
						time.UTC,
					)
					
					// If the resulting date is in the past by more than a month, 
					// assume it's for next year
					if event.Date.Before(time.Now().AddDate(0, -1, 0)) {
						event.Date = time.Date(
							currentYear + 1, 
							parsed.Month(), 
							parsed.Day(), 
							0, 0, 0, 0, 
							time.UTC,
						)
					}
					break
				}
			}
		}
	}
	
	// Extract venue, city, and country
	locationContainer := doc.Find(".field--name-venue")
	if locationContainer.Length() > 0 {
		// Venue name
		venueElem := locationContainer.Find(".field--name-taxonomy-term-title")
		if venueElem.Length() > 0 {
			event.Venue = strings.TrimSpace(venueElem.Text())
		}
		
		// City
		cityElem := locationContainer.Find(".field--name-location .locality")
		if cityElem.Length() > 0 {
			event.City = strings.TrimSpace(cityElem.Text())
		}
		
		// Country
		countryElem := locationContainer.Find(".field--name-location .country")
		if countryElem.Length() > 0 {
			event.Country = strings.TrimSpace(countryElem.Text())
		}
	}
	
	// Determine status based on date
	if !event.Date.IsZero() && event.Date.Before(time.Now()) {
		event.Status = "Completed"
	} else {
		event.Status = "Upcoming"
	}
	
	return event, nil
}