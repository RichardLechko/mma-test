package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"mma-scheduler/internal/models"
)

// UFCEventScraper scrapes events directly from UFC.com
type UFCEventScraper struct {
	client  *http.Client
	baseURL string
}

// NewUFCEventScraper creates a new scraper for UFC events
func NewUFCEventScraper() *UFCEventScraper {
	return &UFCEventScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://www.ufc.com/events",
	}
}

// ScrapeEvents scrapes events from UFC.com
func (s *UFCEventScraper) ScrapeEvents(ctx context.Context) ([]*models.Event, error) {
	var events []*models.Event

	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	// Track processed event names to avoid duplicates
	processedEvents := make(map[string]bool)

	// First, check featured/hero events
	doc.Find(".c-hero--full__event-info").Each(func(i int, heroEvent *goquery.Selection) {
		event := extractEventFromHero(heroEvent)
		if event != nil && !processedEvents[event.Name] {
			processedEvents[event.Name] = true
			events = append(events, event)
		}
	})

	// Then check card events
	doc.Find(".c-card-event--result").Each(func(i int, card *goquery.Selection) {
		event := extractEventFromCard(card)
		if event != nil && !processedEvents[event.Name] {
			processedEvents[event.Name] = true
			events = append(events, event)
		}
	})

	// Advanced filtering to ensure we have future events
	var futureEvents []*models.Event
	now := time.Now()
	for _, event := range events {
		if event.Date.After(now) {
			futureEvents = append(futureEvents, event)
		}
	}

	return futureEvents, nil
}

func extractEventFromCard(card *goquery.Selection) *models.Event {
	// Extract event name
	nameElement := card.Find(".c-card-event--result__headline a")
	name := strings.TrimSpace(nameElement.Text())
	
	// Try to extract name from URL if empty
	if name == "" {
		if href, exists := nameElement.Attr("href"); exists {
			name = extractNameFromURL(href)
		}
	}

	if name == "" || strings.Contains(name, "TBD vs TBD") {
		return nil
	}

	// Extract event URL
	eventURL := ""
	if href, exists := nameElement.Attr("href"); exists {
		eventURL = "https://www.ufc.com" + href
	}

	// Extract date
	dateElement := card.Find(".c-card-event--result__date")
	dateText := strings.TrimSpace(dateElement.Text())
	
	// Parse date
	eventDate, err := parseUFCDate(dateText)
	if err != nil {
		log.Printf("Warning: Could not parse date '%s' for event %s: %v", dateText, name, err)
		return nil
	}

	// Extract venue and location details
	venueElement := card.Find(".field--name-taxonomy-term-title h5")
	venue := strings.TrimSpace(venueElement.Text())

	var city, country string
	card.Find(".field--name-location .address span").Each(func(i int, span *goquery.Selection) {
		spanClass, exists := span.Attr("class")
		if !exists {
			return
		}

		text := strings.TrimSpace(span.Text())
		switch spanClass {
		case "locality":
			city = text
		case "country":
			country = text
		}
	})

	// Determine status
	status := determineEventStatus(eventDate)

	// Create event
	return &models.Event{
		Name:      name,
		Date:      eventDate,
		Venue:     venue,
		City:      city,
		Country:   country,
		Status:    status,
		UFCURL:    eventURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func extractEventFromHero(heroEvent *goquery.Selection) *models.Event {
	// Extract event name
	nameElement := heroEvent.Find(".c-hero--full__event-title")
	name := strings.TrimSpace(nameElement.Text())
	
	// Try to extract name from URL if empty
	if name == "" {
		heroEvent.Find("a").Each(func(i int, a *goquery.Selection) {
			if href, exists := a.Attr("href"); exists && strings.Contains(href, "/event/") {
				name = extractNameFromURL(href)
			}
		})
	}

	if name == "" || strings.Contains(name, "TBD vs TBD") {
		return nil
	}

	// Extract event URL from title link
	eventURL := ""
	heroEvent.Find("a").Each(func(i int, a *goquery.Selection) {
		if href, exists := a.Attr("href"); exists && strings.Contains(href, "/event/") {
			eventURL = "https://www.ufc.com" + href
		}
	})

	// Extract date
	dateElement := heroEvent.Find(".c-hero--full__event-date")
	dateText := strings.TrimSpace(dateElement.Text())
	
	// Parse date
	eventDate, err := parseUFCDate(dateText)
	if err != nil {
		log.Printf("Warning: Could not parse date '%s' for event %s: %v", dateText, name, err)
		return nil
	}

	// Extract venue and location
	locationElement := heroEvent.Find(".c-hero--full__location-city")
	location := strings.TrimSpace(locationElement.Text())

	// Split location into parts
	var city, country string
	locationParts := strings.Split(location, ", ")
	if len(locationParts) >= 1 {
		city = locationParts[0]
	}
	if len(locationParts) >= 2 {
		country = locationParts[1]
	}

	// Extract venue
	venue := ""
	venueElement := heroEvent.Find(".c-hero--full__location-arena")
	if venueElement.Length() > 0 {
		venue = strings.TrimSpace(venueElement.Text())
	}

	// Determine status
	status := determineEventStatus(eventDate)

	// Create event
	return &models.Event{
		Name:      name,
		Date:      eventDate,
		Venue:     venue,
		City:      city,
		Country:   country,
		Status:    status,
		UFCURL:    eventURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
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

// extractNameFromURL extracts an event name from a UFC event URL
func extractNameFromURL(url string) string {
	// Use regex to extract the event name from URL
	urlNameRegex := regexp.MustCompile(`/event/(.+)$`)
	matches := urlNameRegex.FindStringSubmatch(url)
	if len(matches) > 1 {
		urlName := matches[1]
		// Convert URL format to readable name
		urlName = strings.ReplaceAll(urlName, "-", " ")
		urlName = strings.ToUpper(urlName)
		// Improve formatting of UFC numbers
		urlName = regexp.MustCompile(`UFC (\d+)`).ReplaceAllString(urlName, "UFC $1")
		return urlName
	}
	return ""
}