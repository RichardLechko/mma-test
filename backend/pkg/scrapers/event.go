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

func (s *UFCEventScraper) ScrapeEvents(ctx context.Context) ([]*models.Event, error) {
	var allEvents []*models.Event
	processedEvents := make(map[string]bool)
	var mu sync.Mutex
	
	page := 0
	consecutiveEmptyPages := 0
	maxConsecutiveEmpty := 5
	
	for consecutiveEmptyPages < maxConsecutiveEmpty {
		select {
		case <-ctx.Done():
			return allEvents, ctx.Err()
		default:
		}
		
		pageURL := s.baseURL
		if page > 0 {
			pageURL = fmt.Sprintf("%s?page=%d", s.baseURL, page)
		}
		
		log.Printf("Processing page %d", page)
		events, err := s.processPage(ctx, pageURL)
		if err != nil {
			log.Printf("Error processing page %d: %v", page, err)
			consecutiveEmptyPages++
			page++
			continue
		}
		
		if len(events) == 0 {
			consecutiveEmptyPages++
			log.Printf("Page %d empty, consecutive empty pages: %d/%d", 
				page, consecutiveEmptyPages, maxConsecutiveEmpty)
		} else {
			consecutiveEmptyPages = 0
			log.Printf("Page %d found %d events", page, len(events))
			
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
		
		time.Sleep(250 * time.Millisecond)
	}
	
	log.Printf("Scraping completed. Found %d events", len(allEvents))
	return allEvents, nil
}

func (s *UFCEventScraper) processPage(ctx context.Context, pageURL string) ([]*models.Event, error) {
	select {
	case <-s.rateLimiter:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	
	client := s.getClient()
	
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}
	
	var events []*models.Event
	
	doc.Find(".c-card-event--result").Each(func(i int, card *goquery.Selection) {
		linkElem := card.Find(".c-card-event--result__headline a")
		eventURL, exists := linkElem.Attr("href")
		if !exists || eventURL == "" {
			return
		}
		
		if !strings.HasPrefix(eventURL, "http") {
			eventURL = "https://www.ufc.com" + eventURL
		}
		
		eventName := strings.TrimSpace(linkElem.Text())
		
		event := &models.Event{
			UFCURL: eventURL,
			Name:   eventName,
		}
		
		dateElem := card.Find(".c-card-event--result__date")
		dateText := strings.TrimSpace(dateElem.Text())
		
		timestampStr, exists := dateElem.Attr("data-main-card-timestamp")
		if exists {
			timestampInt := 0
			fmt.Sscanf(timestampStr, "%d", &timestampInt)
			if timestampInt > 0 {
				event.Date = time.Unix(int64(timestampInt), 0).UTC()
			}
		}
		
		if event.Date.IsZero() {
			dateText = strings.Split(dateText, " / ")[0]
			dateFormats := []string{
				"Mon, Jan 2",
				"Monday, January 2",
			}
			
			for _, format := range dateFormats {
				parsed, err := time.Parse(format, dateText)
				if err == nil {
					currentYear := time.Now().Year()
					event.Date = time.Date(
						currentYear, 
						parsed.Month(), 
						parsed.Day(), 
						0, 0, 0, 0, 
						time.UTC,
					)
					
					if event.Date.Before(time.Now().UTC().AddDate(0, -1, 0)) {
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
		
		locationElem := card.Find(".c-card-event--result__location")
		
		venueElem := locationElem.Find(".field--name-taxonomy-term-title h5")
		if venueElem.Length() > 0 {
			event.Venue = strings.TrimSpace(venueElem.Text())
		}
		
		cityElem := locationElem.Find(".field--name-location .locality")
		if cityElem.Length() > 0 {
			event.City = strings.TrimSpace(cityElem.Text())
		}
		
		countryElem := locationElem.Find(".field--name-location .country")
		if countryElem.Length() > 0 {
			event.Country = strings.TrimSpace(countryElem.Text())
		}
		
		if !event.Date.IsZero() && event.Date.Before(time.Now().UTC()) {
			event.Status = "Completed"
		} else {
			event.Status = "Upcoming"
		}
		
		if event.Date.IsZero() || event.Venue == "" || event.City == "" || event.Country == "" {
			detailEvent, err := s.scrapeEventDetails(ctx, eventURL)
			if err == nil && detailEvent != nil {
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
		
		if event.UFCURL != "" && (event.Venue != "" || event.City != "" || event.Country != "" || !event.Date.IsZero()) {
			events = append(events, event)
		}
	})
	
	return events, nil
}

func (s *UFCEventScraper) scrapePage(
	ctx context.Context, 
	page int, 
	detailSem chan struct{},
	detailWg *sync.WaitGroup,
	eventDetailChan chan<- EventDetail,
) int {
	select {
	case <-s.rateLimiter:
	case <-ctx.Done():
		return 0
	}

	pageURL := s.baseURL
	if page > 0 {
		pageURL = fmt.Sprintf("%s?page=%d", s.baseURL, page)
	}

	client := s.getClient()
	
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		log.Printf("Error creating request for page %d: %v", page, err)
		return 0
	}
	
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error fetching page %d: %v", page, err)
		return 0
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("Bad status code for page %d: %d", page, resp.StatusCode)
		return 0
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("Error parsing HTML for page %d: %v", page, err)
		return 0
	}

	eventsFound := 0
	
	doc.Find(".c-card-event--result").Each(func(i int, card *goquery.Selection) {
		linkElem := card.Find(".c-card-event--result__headline a")
		eventURL, exists := linkElem.Attr("href")
		if !exists || eventURL == "" {
			return
		}

		if !strings.HasPrefix(eventURL, "http") {
			eventURL = "https://www.ufc.com" + eventURL
		}

		eventName := strings.TrimSpace(linkElem.Text())
		
		event, basicInfoFound := s.extractBasicEventInfo(card, eventURL)
		
		needDetails := !basicInfoFound
		
		if needDetails {
			detailWg.Add(1)
			go func(url, name string, baseEvent *models.Event) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Recovered from panic in detail scraper for %s: %v", name, r)
					}
					detailWg.Done()
				}()
				
				select {
				case detailSem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				
				defer func() {
					select {
					case <-detailSem:
					default:
					}
				}()
				
				select {
				case <-s.rateLimiter:
				case <-ctx.Done():
					return
				}
				
				detailCtx, detailCancel := context.WithTimeout(ctx, 30*time.Second)
				defer detailCancel()
				
				detailEvent, err := s.scrapeEventDetails(detailCtx, url)
				
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				var result EventDetail
				
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
					detailEvent.UFCURL = url
					result = EventDetail{
						URL:     url,
						Name:    name,
						Event:   detailEvent,
						PageNum: page,
					}
				} else {
					result = EventDetail{
						URL:     url,
						Name:    name,
						Error:   err,
						PageNum: page,
					}
				}
				
				select {
				case eventDetailChan <- result:
				case <-ctx.Done():
				}
			}(eventURL, eventName, event)
		} else if event != nil {
			select {
			case eventDetailChan <- EventDetail{
				URL:     eventURL,
				Name:    eventName,
				Event:   event,
				PageNum: page,
			}:
			case <-ctx.Done():
				return
			}
		}
		
		eventsFound++
	})

	return eventsFound
}

func (s *UFCEventScraper) extractBasicEventInfo(card *goquery.Selection, eventURL string) (*models.Event, bool) {
	event := &models.Event{
		UFCURL: eventURL,
	}
	
	completeInfo := false
	
	dateElem := card.Find(".c-card-event--result__date")
	dateText := strings.TrimSpace(dateElem.Text())
	
	timestampStr, exists := dateElem.Attr("data-main-card-timestamp")
	if exists {
		timestampInt := 0
		fmt.Sscanf(timestampStr, "%d", &timestampInt)
		if timestampInt > 0 {
			event.Date = time.Unix(int64(timestampInt), 0).UTC()
		}
	}
	
	if event.Date.IsZero() {
		dateText = strings.Split(dateText, " / ")[0]
		dateFormats := []string{
			"Mon, Jan 2",
			"Monday, January 2",
		}
		
		for _, format := range dateFormats {
			parsed, err := time.Parse(format, dateText)
			if err == nil {
				currentYear := time.Now().Year()
				event.Date = time.Date(
					currentYear, 
					parsed.Month(), 
					parsed.Day(), 
					0, 0, 0, 0, 
					time.UTC,
				)
				
				if event.Date.Before(time.Now().UTC().AddDate(0, -1, 0)) {
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
	
	locationElem := card.Find(".c-card-event--result__location")
	
	venueElem := locationElem.Find(".field--name-taxonomy-term-title h5")
	if venueElem.Length() > 0 {
		event.Venue = strings.TrimSpace(venueElem.Text())
	}
	
	cityElem := locationElem.Find(".field--name-location .locality")
	if cityElem.Length() > 0 {
		event.City = strings.TrimSpace(cityElem.Text())
	}
	
	countryElem := locationElem.Find(".field--name-location .country")
	if countryElem.Length() > 0 {
		event.Country = strings.TrimSpace(countryElem.Text())
	}
	
	if !event.Date.IsZero() && event.Venue != "" && event.City != "" && event.Country != "" {
		completeInfo = true
	}
	
	if !event.Date.IsZero() && event.Date.Before(time.Now().UTC()) {
		event.Status = "Completed"
	} else {
		event.Status = "Upcoming"
	}
	
	return event, completeInfo
}

func (s *UFCEventScraper) scrapeEventDetails(ctx context.Context, eventURL string) (*models.Event, error) {
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
	
	dateContainer := doc.Find(".hero-event-results__date-container")
	if dateContainer.Length() > 0 {
		timestampStr, exists := dateContainer.Attr("data-main-card-timestamp")
		if exists {
			timestampInt := 0
			fmt.Sscanf(timestampStr, "%d", &timestampInt)
			if timestampInt > 0 {
				event.Date = time.Unix(int64(timestampInt), 0).UTC()
			}
		}
		
		if event.Date.IsZero() {
			dateText := strings.TrimSpace(dateContainer.Text())
			dateFormats := []string{
				"Mon, Jan 2",
				"Monday, January 2",
			}
			
			for _, format := range dateFormats {
				parsed, err := time.Parse(format, dateText)
				if err == nil {
					currentYear := time.Now().Year()
					event.Date = time.Date(
						currentYear, 
						parsed.Month(), 
						parsed.Day(), 
						0, 0, 0, 0, 
						time.UTC,
					)
					
					if event.Date.Before(time.Now().UTC().AddDate(0, -1, 0)) {
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
	
	locationContainer := doc.Find(".field--name-venue")
	if locationContainer.Length() > 0 {
		venueElem := locationContainer.Find(".field--name-taxonomy-term-title")
		if venueElem.Length() > 0 {
			event.Venue = strings.TrimSpace(venueElem.Text())
		}
		
		cityElem := locationContainer.Find(".field--name-location .locality")
		if cityElem.Length() > 0 {
			event.City = strings.TrimSpace(cityElem.Text())
		}
		
		countryElem := locationContainer.Find(".field--name-location .country")
		if countryElem.Length() > 0 {
			event.Country = strings.TrimSpace(countryElem.Text())
		}
	}
	
	if !event.Date.IsZero() && event.Date.Before(time.Now().UTC()) {
		event.Status = "Completed"
	} else {
		event.Status = "Upcoming"
	}
	
	return event, nil
}

// ScrapeRecentEvents scrapes all future events plus events from the last X months
func (s *UFCEventScraper) ScrapeRecentEvents(ctx context.Context, cutoffDate time.Time) ([]*models.Event, error) {
    var recentEvents []*models.Event
    processedEvents := make(map[string]bool)
    var mu sync.Mutex
    
    page := 0
    consecutiveEmptyPages := 0
    maxConsecutiveEmpty := 3
    stopScraping := false
    foundPastCutoffEvents := false  // Flag to track if we've found events past cutoff
    
    for consecutiveEmptyPages < maxConsecutiveEmpty && !stopScraping {
        select {
        case <-ctx.Done():
            return recentEvents, ctx.Err()
        default:
        }
        
        pageURL := s.baseURL
        if page > 0 {
            pageURL = fmt.Sprintf("%s?page=%d", s.baseURL, page)
        }
        
        log.Printf("Processing page %d", page)
        events, err := s.processPage(ctx, pageURL)
        if err != nil {
            log.Printf("Error processing page %d: %v", page, err)
            consecutiveEmptyPages++
            page++
            continue
        }
        
        if len(events) == 0 {
            consecutiveEmptyPages++
            log.Printf("Page %d empty, consecutive empty pages: %d/%d", 
                page, consecutiveEmptyPages, maxConsecutiveEmpty)
        } else {
            consecutiveEmptyPages = 0
            log.Printf("Page %d found %d events", page, len(events))
            
            currentTime := time.Now()
            pageHasFutureEvents := false
            allEventsBeforeCutoff := true
            
            // First pass: Check if this page has ANY future events or if ALL events are before cutoff
            for _, event := range events {
                if event.Date.After(currentTime) {
                    pageHasFutureEvents = true
                }
                if event.Date.After(cutoffDate) || event.Date.Equal(cutoffDate) {
                    allEventsBeforeCutoff = false
                }
            }
            
            // If this page has no future events AND we've already found past cutoff events,
            // we can stop scraping
            if !pageHasFutureEvents && foundPastCutoffEvents && allEventsBeforeCutoff {
                log.Printf("Page %d has no future events and all events are before cutoff - stopping scrape", page)
                stopScraping = true
                break
            }
            
            // Process events on this page
            for _, event := range events {
                // Log the event
                log.Printf("Found event: %s on %s at %s",
                    event.Name,
                    event.Date.Format("2006-01-02"),
                    event.Venue)
                
                // Keep future events and recent past events
                if event.Date.After(currentTime) || event.Date.After(cutoffDate) || event.Date.Equal(cutoffDate) {
                    mu.Lock()
                    if !processedEvents[event.UFCURL] {
                        recentEvents = append(recentEvents, event)
                        processedEvents[event.UFCURL] = true
                    }
                    mu.Unlock()
                    
                    if event.Date.Before(currentTime) {
                        foundPastCutoffEvents = true
                    }
                } else {
                    // If all events on the page are past events and we've already found
                    // some past events that are after the cutoff, we can stop
                    if foundPastCutoffEvents && !pageHasFutureEvents {
                        log.Printf("Found event older than cutoff date (%s): %s on %s - stopping scrape",
                            cutoffDate.Format("2006-01-02"),
                            event.Name,
                            event.Date.Format("2006-01-02"))
                        stopScraping = true
                        break
                    }
                }
            }
        }
        
        page++
        time.Sleep(250 * time.Millisecond)
    }
    
    log.Printf("Recent event scraping completed. Found %d events", len(recentEvents))
    return recentEvents, nil
}