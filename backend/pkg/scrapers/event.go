package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"mma-scheduler/internal/models"
)

// UFCEventScraper scrapes event information from UFC.com
type UFCEventScraper struct {
	client  *http.Client
	baseURL string
}

// NewUFCEventScraper creates a new scraper for UFC events
func NewUFCEventScraper() *UFCEventScraper {
	return &UFCEventScraper{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: "https://www.ufc.com/events",
	}
}

// ScrapeEvents scrapes events from UFC.com, focusing on UFC URL, event date, venue, city, and country
func (s *UFCEventScraper) ScrapeEvents(ctx context.Context) ([]*models.Event, error) {
	var allEvents []*models.Event
	processedEvents := make(map[string]bool)
	consecutiveEmptyPages := 0
	page := 0

	for consecutiveEmptyPages < 5 {
		select {
		case <-ctx.Done():
			return allEvents, ctx.Err()
		default:
			// Construct URL with pagination
			pageURL := s.baseURL
			if page > 0 {
				pageURL = fmt.Sprintf("%s?page=%d", s.baseURL, page)
			}

			req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
			if err != nil {
				log.Printf("Error creating request for page %d: %v", page, err)
				page++
				continue
			}
			
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
			
			resp, err := s.client.Do(req)
			if err != nil {
				log.Printf("Error fetching page %d: %v", page, err)
				page++
				consecutiveEmptyPages++
				continue
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				log.Printf("Bad status code for page %d: %d", page, resp.StatusCode)
				page++
				consecutiveEmptyPages++
				continue
			}
			
			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				log.Printf("Error parsing HTML for page %d: %v", page, err)
				page++
				consecutiveEmptyPages++
				continue
			}

			// Track if we found any events on this page
			pageEvents := 0

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

				// Skip if we've already processed this event
				if processedEvents[eventURL] {
					return
				}

				// Extract event name for logging
				eventName := strings.TrimSpace(linkElem.Text())
				
				// Extract date information
				dateElem := card.Find(".c-card-event--result__date")
				dateText := strings.TrimSpace(dateElem.Text())
				
				// Try to get timestamp from data attribute first
				var eventDate time.Time
				timestampStr, exists := dateElem.Attr("data-main-card-timestamp")
				if exists {
					// Convert Unix timestamp to time.Time
					timestampInt := 0
					fmt.Sscanf(timestampStr, "%d", &timestampInt)
					if timestampInt > 0 {
						// Use the original timestamp with correct date and time
						eventDate = time.Unix(int64(timestampInt), 0)
						log.Printf("Parsed timestamp %d to date: %s", timestampInt, eventDate.Format(time.RFC3339))
					}
				}
				
				// If timestamp failed, try to parse the text date
				if eventDate.IsZero() {
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
							eventDate = time.Date(
								currentYear, 
								parsed.Month(), 
								parsed.Day(), 
								0, 0, 0, 0, 
								time.UTC,
							)
							
							// If the resulting date is in the past by more than a month, 
							// assume it's for next year
							if eventDate.Before(time.Now().AddDate(0, -1, 0)) {
								eventDate = time.Date(
									currentYear + 1, 
									parsed.Month(), 
									parsed.Day(), 
									0, 0, 0, 0, 
									time.UTC,
								)
							}
							
							log.Printf("Parsed text date to: %s (guessed year)", eventDate.Format(time.RFC3339))
							break
						}
					}
				}
				
				// Extract location information
				var venue, city, country string
				
				locationElem := card.Find(".c-card-event--result__location")
				
				// Extract venue
				venueElem := locationElem.Find(".field--name-taxonomy-term-title h5")
				if venueElem.Length() > 0 {
					venue = strings.TrimSpace(venueElem.Text())
				}
				
				// Extract city and country
				cityElem := locationElem.Find(".field--name-location .locality")
				if cityElem.Length() > 0 {
					city = strings.TrimSpace(cityElem.Text())
				}
				
				countryElem := locationElem.Find(".field--name-location .country")
				if countryElem.Length() > 0 {
					country = strings.TrimSpace(countryElem.Text())
				}
				
				// For incomplete data, fetch the event page itself for more details
				if eventDate.IsZero() || venue == "" || city == "" || country == "" {
					eventDetails, err := s.scrapeEventDetails(ctx, eventURL)
					if err == nil {
						if eventDate.IsZero() && !eventDetails.Date.IsZero() {
							eventDate = eventDetails.Date
						}
						if venue == "" && eventDetails.Venue != "" {
							venue = eventDetails.Venue
						}
						if city == "" && eventDetails.City != "" {
							city = eventDetails.City
						}
						if country == "" && eventDetails.Country != "" {
							country = eventDetails.Country
						}
					}
				}
				
				// Determine status based on date
				status := "Upcoming"
				if !eventDate.IsZero() && eventDate.Before(time.Now()) {
					status = "Completed"
				}
				
				// Create event with the essential fields
				event := &models.Event{
					Date:      eventDate,
					Venue:     venue,
					City:      city,
					Country:   country,
					UFCURL:    eventURL,
					Status:    status,
				}
				
				// Add event if we have the UFC URL plus at least one other piece of data
				if eventURL != "" && (venue != "" || city != "" || country != "" || !eventDate.IsZero()) {
					allEvents = append(allEvents, event)
					processedEvents[eventURL] = true
					pageEvents++
					
					// Log event
					log.Printf("Found event: %s on %s at %s", eventName, eventDate.Format("2006-01-02"), venue)
				}
			})

			// Reset consecutive empty pages if we found events
			if pageEvents > 0 {
				consecutiveEmptyPages = 0
			} else {
				consecutiveEmptyPages++
			}

			page++
		}
	}

	return allEvents, nil
}

// scrapeEventDetails fetches additional details from an event's page
func (s *UFCEventScraper) scrapeEventDetails(ctx context.Context, eventURL string) (*models.Event, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", eventURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
	
	resp, err := s.client.Do(req)
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
				log.Printf("Event details timestamp %d parsed to: %s", timestampInt, event.Date.Format(time.RFC3339))
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
					
					log.Printf("Parsed event details date to: %s (guessed year)", event.Date.Format(time.RFC3339))
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