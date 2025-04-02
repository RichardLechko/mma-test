package services

import (
	"context"
	"fmt"
	"time"

	"mma-scheduler/internal/models"
	"mma-scheduler/pkg/scrapers"
)

type ScraperService struct {
	eventScraper *scrapers.EventScraper
	proxyManager *scrapers.ProxyManager
}

func NewScraperService() *ScraperService {
	config := scrapers.DefaultConfig()
	proxies := []string{}
	proxyManager := scrapers.NewProxyManager(proxies, 5*time.Minute)

	return &ScraperService{
		eventScraper: scrapers.NewEventScraper(config),
		proxyManager: proxyManager,
	}
}

// convertWikiEventToModel converts a WikiEvent to a models.Event
func convertWikiEventToModel(wikiEvent *scrapers.WikiEvent) models.Event {
	return models.Event{
		Name:      wikiEvent.Name,
		Date:      wikiEvent.Date,
		Location:  fmt.Sprintf("%s, %s, %s", wikiEvent.Venue, wikiEvent.City, wikiEvent.Country),
		Promotion: "UFC",
		MainCard:  []models.Fight{},  // To be populated separately if needed
		PrelimCard: []models.Fight{}, // To be populated separately if needed
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *ScraperService) ScrapeEvent(ctx context.Context, url string) (*models.Event, error) {
	// For single event scraping, we'll need to implement this in the EventScraper
	return nil, fmt.Errorf("single event scraping not implemented for Wikipedia source")
}

func (s *ScraperService) ScrapeUpcomingEvents(ctx context.Context) ([]models.Event, error) {
	wikiEvents, err := s.eventScraper.ScrapeUpcomingEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape upcoming events: %w", err)
	}

	// Convert WikiEvents to models.Events
	var events []models.Event
	for _, wikiEvent := range wikiEvents {
		event := convertWikiEventToModel(wikiEvent)
		events = append(events, event)
	}

	return events, nil
}

func (s *ScraperService) UpdateProxies(proxies []string) {
	s.proxyManager = scrapers.NewProxyManager(proxies, 5*time.Minute)
}

func (s *ScraperService) GetNextProxy() (string, error) {
	return s.proxyManager.GetProxy()
}