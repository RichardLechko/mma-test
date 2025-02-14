package scrapers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mma-scheduler/internal/models"
	"mma-scheduler/pkg/databases"

	"github.com/PuerkitoBio/goquery"
)

type EventScraper struct {
	*BaseScraper
}

func NewEventScraper(config ScraperConfig) *EventScraper {
	return &EventScraper{
		BaseScraper: NewBaseScraper(config),
	}
}

func (s *EventScraper) ScrapeEvents() ([]*databases.Event, error) {
	var allEvents []*databases.Event
	currentYear := time.Now().Year()

	for year := currentYear; year >= 1993; year-- {
		events, err := s.scrapeEventsPage(year)
		if err != nil {
			return nil, fmt.Errorf("error scraping year %d: %v", year, err)
		}

		if len(events) > 0 {
			for _, event := range events {
				if number, ok := extractPPVNumber(event.Name); ok && number < 229 {
					log.Printf("Reached UFC 229 cutoff point. Stopping scraper.")
					return allEvents, nil
				}
				allEvents = append(allEvents, event)
			}
		}

		time.Sleep(time.Second * 2)
	}

	return allEvents, nil
}

func (s *EventScraper) scrapeEventsPage(year int) ([]*databases.Event, error) {
	url := fmt.Sprintf("https://www.espn.com/mma/schedule/_/year/%d/league/ufc", year)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/123.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	var events []*databases.Event

	doc.Find("tr.Table__TR").Each(func(i int, tr *goquery.Selection) {
		dateText := strings.TrimSpace(tr.Find("td.date__col span.date__innerCell").Text())
		eventName := strings.TrimSpace(tr.Find("td.event__col a.AnchorLink").Text())
		location := strings.TrimSpace(tr.Find("td.location__col div").Text())

		if eventName == "" || !strings.HasPrefix(eventName, "UFC") {
			return
		}

		var dateStr string
		parsedDate, err := time.Parse("Jan 2", dateText)
		if err != nil {
			parsedDate, err = time.Parse("Jan. 2", dateText)
			if err != nil {
				parsedDate = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
				log.Printf("Warning: Could not parse date '%s' for event '%s', using fallback date", dateText, eventName)
			}
		}

		dateStr = time.Date(year, parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02 15:04:05 -0700 MST")

		mainEvent := tr.Find("td.event__col div.matchup").Text()
		if mainEvent == "" {
			mainEvent = "TBD"
		}

		if location == "" {
			location = "TBD"
		}

		event := &databases.Event{
			Name:      eventName,
			Date:      dateStr,
			Location:  location,
			MainEvent: mainEvent,
		}

		events = append(events, event)
	})

	return events, nil
}

func (s *EventScraper) ScrapeEvent(ctx context.Context, url string) (*models.Event, error) {
	event := &models.Event{
		Promotion: "UFC",
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching event: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	doc.Find(".event-header").Each(func(i int, s *goquery.Selection) {
		event.Name = strings.TrimSpace(s.Find(".event-title").Text())

		dateStr := s.Find(".event-date").Text()
		if parsedDate, err := time.Parse("January 2, 2006", dateStr); err == nil {
			event.Date = parsedDate
		}

		venue := strings.TrimSpace(s.Find(".event-venue").Text())
		city := strings.TrimSpace(s.Find(".event-city").Text())
		country := strings.TrimSpace(s.Find(".event-country").Text())

		var locationParts []string
		if venue != "" {
			locationParts = append(locationParts, venue)
		}
		if city != "" {
			locationParts = append(locationParts, city)
		}
		if country != "" {
			locationParts = append(locationParts, country)
		}
		event.Location = strings.Join(locationParts, ", ")
	})

	doc.Find(".fight-card .main-card .fight").Each(func(i int, s *goquery.Selection) {
		fight := models.Fight{
			EventID:     event.ID,
			WeightClass: strings.TrimSpace(s.Find(".weight-class").Text()),
			Fighter1:    strings.TrimSpace(s.Find(".fighter-1").Text()),
			Fighter2:    strings.TrimSpace(s.Find(".fighter-2").Text()),
			IsMainEvent: i == 0, // First fight in main card is main event
			Order:       i,
		}
		event.MainCard = append(event.MainCard, fight)
	})

	doc.Find(".fight-card .prelim-card .fight").Each(func(i int, s *goquery.Selection) {
		fight := models.Fight{
			EventID:     event.ID,
			WeightClass: strings.TrimSpace(s.Find(".weight-class").Text()),
			Fighter1:    strings.TrimSpace(s.Find(".fighter-1").Text()),
			Fighter2:    strings.TrimSpace(s.Find(".fighter-2").Text()),
			IsMainEvent: false,
			Order:       i,
		}
		event.PrelimCard = append(event.PrelimCard, fight)
	})

	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()

	return event, nil
}

func extractPPVNumber(name string) (int, bool) {
	patterns := []string{
		`UFC (\d+)`,
		`UFC(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(name); len(matches) > 1 {
			if num, err := strconv.Atoi(matches[1]); err == nil {
				return num, true
			}
		}
	}

	return 0, false
}

func (s *EventScraper) ScrapeUpcomingEvents(ctx context.Context) ([]models.Event, error) {
	var events []models.Event
	dbEvents, err := s.scrapeEventsPage(0)
	if err != nil {
		return nil, err
	}

	for _, dbEvent := range dbEvents {
		parsedDate, err := time.Parse("2006-01-02 15:04:05 -0700 MST", dbEvent.Date)
		if err != nil {
			fmt.Printf("Failed to parse date for event %s: %v\n", dbEvent.Name, err)
			continue
		}

		event := models.Event{
			Name:      dbEvent.Name,
			Date:      parsedDate,
			Location:  dbEvent.Location,
			Promotion: "UFC",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		events = append(events, event)
	}

	return events, nil
}
