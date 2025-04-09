package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// UFCEventScraper scrapes events directly from UFC.com
type UFCEventScraper struct {
	*BaseScraper
	baseURL string
}

// NewUFCEventScraper creates a new scraper for UFC events
func NewUFCEventScraper(config ScraperConfig) *UFCEventScraper {
	return &UFCEventScraper{
		BaseScraper: NewBaseScraper(config),
		baseURL:     "https://www.ufc.com/events",
	}
}

// UFCEvent represents an event from UFC.com
type UFCEvent struct {
	Name      string
	Date      time.Time
	Venue     string
	Location  string
	EventType string
	Status    string
	UFCURL    string
}

// ScrapeEvents scrapes all events from UFC.com with pagination
func (s *UFCEventScraper) ScrapeEvents(ctx context.Context, maxPages int) ([]*UFCEvent, error) {
	var allEvents []*UFCEvent
	
	// Start with the first page (upcoming events)
	page := 0
	for page < maxPages {
		pageURL := s.baseURL
		if page > 0 {
			pageURL = fmt.Sprintf("%s?page=%d", s.baseURL, page)
		}
		
		log.Printf("Scraping UFC events from page %d: %s", page, pageURL)
		
		events, hasMorePages, err := s.scrapeEventPage(ctx, pageURL)
		if err != nil {
			log.Printf("Error scraping page %d: %v", page, err)
			return allEvents, nil // Return what we have so far
		}
		
		log.Printf("Found %d events on page %d", len(events), page)
		allEvents = append(allEvents, events...)
		
		if !hasMorePages {
			log.Printf("No more pages found after page %d", page)
			break
		}
		
		page++
		
		// Add a small delay between requests to avoid overwhelming the server
		select {
		case <-ctx.Done():
			return allEvents, ctx.Err()
		case <-time.After(1 * time.Second):
			// Continue to next page
		}
	}
	
	log.Printf("Total events scraped: %d", len(allEvents))
	return allEvents, nil
}

// scrapeEventPage scrapes a single page of events
func (s *UFCEventScraper) scrapeEventPage(ctx context.Context, url string) ([]*UFCEvent, bool, error) {
	var events []*UFCEvent
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("error creating request: %v", err)
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("error fetching page: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("error parsing HTML: %v", err)
	}
	
	// Check if there's a next page
	hasMorePages := doc.Find(".pager__item--next a").Length() > 0
	
	// Find all event cards
	doc.Find(".view-content .view-display-id-events_card, .view-content .view-display-id-past_events").Find(".l-listing__item").Each(func(i int, card *goquery.Selection) {
		event := &UFCEvent{}
		
		// Extract event URL
		eventLink := card.Find("a.e-button--ticket, a.card-title-link")
		if eventHref, exists := eventLink.Attr("href"); exists {
			if strings.HasPrefix(eventHref, "/event/") {
				event.UFCURL = "https://www.ufc.com" + eventHref
			} else if strings.HasPrefix(eventHref, "https://") {
				event.UFCURL = eventHref
			}
		}
		
		// Skip if no URL found
		if event.UFCURL == "" {
			return
		}
		
		// Extract event name
		titleEl := card.Find(".c-card-event--result__headline")
		if titleEl.Length() == 0 {
			titleEl = card.Find(".c-card-event--result__date")
		}
		if titleEl.Length() == 0 {
			titleEl = card.Find(".c-event-fight-card-broadcaster__headline")
		}
		
		eventName := strings.TrimSpace(titleEl.Text())
		
		// Also try to get the event name from the URL if possible
		if eventName == "" || strings.Contains(eventName, "TBD vs TBD") {
			urlNameRegex := regexp.MustCompile(`/event/(.+)$`)
			matches := urlNameRegex.FindStringSubmatch(event.UFCURL)
			if len(matches) > 1 {
				urlName := matches[1]
				// Convert URL format to readable name
				urlName = strings.ReplaceAll(urlName, "-", " ")
				urlName = strings.ToUpper(urlName)
				// Improve formatting of UFC numbers
				urlName = regexp.MustCompile(`UFC (\d+)`).ReplaceAllString(urlName, "UFC $1")
				eventName = urlName
			}
		}
		
		// Skip events without proper names or TBD vs TBD
		if eventName == "" || strings.Contains(eventName, "TBD vs TBD") {
			return
		}
		
		event.Name = eventName
		
		// Extract event date
		dateEl := card.Find(".c-card-event--result__date")
		if dateEl.Length() == 0 {
			dateEl = card.Find(".c-event-fight-card-broadcaster__date")
		}
		
		dateStr := strings.TrimSpace(dateEl.Text())
		if dateStr != "" {
			// Try to parse the date
			parsedDate, err := parseUFCDate(dateStr)
			if err != nil {
				log.Printf("Warning: Could not parse date '%s' for event %s: %v", 
					dateStr, event.Name, err)
			} else {
				event.Date = parsedDate
			}
		}
		
		// Extract venue and location
		venueLocationEl := card.Find(".field--name-venue")
		if venueLocationEl.Length() == 0 {
			venueLocationEl = card.Find(".c-event-fight-card-broadcaster__location")
		}
		
		venueLocationStr := strings.TrimSpace(venueLocationEl.Text())
		if venueLocationStr != "" {
			// Try to split venue and location
			event.Venue, event.Location = extractVenueAndLocation(venueLocationStr)
		}
		
		// Extract event type
		eventTypeEl := card.Find(".c-card-event--result__banner-tag")
		if eventTypeEl.Length() > 0 {
			event.EventType = strings.TrimSpace(eventTypeEl.Text())
		}
		
		// Determine status based on date
		event.Status = determineEventStatus(event.Date)
		
		// Only add events with proper names
		if event.Name != "" && !strings.Contains(event.Name, "TBD vs TBD") {
			events = append(events, event)
		}
	})
	
	return events, hasMorePages, nil
}

// parseUFCDate tries to parse the date string from UFC website
func parseUFCDate(dateStr string) (time.Time, error) {
	// Clean up the date string
	dateStr = strings.TrimSpace(dateStr)
	dateStr = regexp.MustCompile(`\s+`).ReplaceAllString(dateStr, " ")
	
	// Extract just the date portion if there's additional text
	dateParts := strings.Split(dateStr, "|")
	if len(dateParts) > 1 {
		dateStr = strings.TrimSpace(dateParts[0])
	}
	
	// Try various date formats
	formats := []string{
		"Jan 2, 2006",
		"January 2, 2006",
		"Jan. 2, 2006",
		"Monday, Jan 2, 2006",
		"Monday, January 2, 2006",
		"Jan 2",
		"January 2",
		"2006-01-02",
		"01/02/2006",
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			// If year is missing, use current year
			if t.Year() == 0 {
				currentYear := time.Now().Year()
				t = time.Date(currentYear, t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			}
			return t, nil
		}
	}
	
	// Try to extract with regex as a last resort
	dateRegex := regexp.MustCompile(`(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z.]*\s+(\d{1,2})(?:,?\s+(\d{4}))?`)
	matches := dateRegex.FindStringSubmatch(dateStr)
	if len(matches) >= 3 {
		monthMap := map[string]time.Month{
			"Jan": time.January, "Feb": time.February, "Mar": time.March,
			"Apr": time.April, "May": time.May, "Jun": time.June,
			"Jul": time.July, "Aug": time.August, "Sep": time.September,
			"Oct": time.October, "Nov": time.November, "Dec": time.December,
		}
		
		month := monthMap[matches[1]]
		day := 1
		fmt.Sscanf(matches[2], "%d", &day)
		
		year := time.Now().Year()
		if len(matches) >= 4 && matches[3] != "" {
			fmt.Sscanf(matches[3], "%d", &year)
		}
		
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), nil
	}
	
	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}

// extractVenueAndLocation extracts venue and location from a combined string
func extractVenueAndLocation(venueLocationStr string) (string, string) {
	// Look for a pattern with the venue followed by location
	// Example: "T-Mobile Arena | Las Vegas, NV"
	parts := strings.Split(venueLocationStr, "|")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	
	// Look for a pattern with venue followed by city and state
	parts = strings.Split(venueLocationStr, ",")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(strings.Join(parts[1:], ","))
	}
	
	// If we can't determine which is which, return the whole string as location
	return "", venueLocationStr
}

// determineEventStatus determines the event status based on date
func determineEventStatus(eventDate time.Time) string {
	if eventDate.IsZero() {
		return "Scheduled" // Default status if no date
	}
	
	now := time.Now()
	
	if eventDate.After(now) {
		return "Scheduled"
	} else {
		return "Completed"
	}
}

// SaveEvents saves the events to the database
func (s *UFCEventScraper) SaveEvents(ctx context.Context, db *sql.DB, events []*UFCEvent) (int, error) {
	insertCount := 0
	
	for _, event := range events {
		query := `
        INSERT INTO events (
            name, event_date, venue, location, event_type, status, ufc_url,
            created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9
        )
        ON CONFLICT (name) 
        DO UPDATE SET
            event_date = EXCLUDED.event_date,
            venue = EXCLUDED.venue,
            location = EXCLUDED.location,
            event_type = EXCLUDED.event_type,
            status = EXCLUDED.status,
            ufc_url = EXCLUDED.ufc_url,
            updated_at = EXCLUDED.updated_at
        RETURNING id`
		
		now := time.Now()
		var eventID string
		
		err := db.QueryRowContext(ctx, query,
			event.Name,
			event.Date,
			event.Venue,
			event.Location,
			event.EventType,
			event.Status,
			event.UFCURL,
			now,
			now,
		).Scan(&eventID)
		
		if err != nil {
			log.Printf("Failed to save event %s: %v", event.Name, err)
			continue
		}
		
		insertCount++
	}
	
	return insertCount, nil
}