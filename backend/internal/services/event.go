package services

import (
	"context"
	"fmt"
	"time"
	"mma-scheduler/internal/models"
	"mma-scheduler/pkg/scrapers"
)

// ScraperService provides access to various scrapers
type ScraperService struct {
	ufcEventScraper *scrapers.UFCEventScraper
	proxyManager    *scrapers.ProxyManager
}

// NewScraperService creates a new scraper service
func NewScraperService() *ScraperService {
	config := scrapers.DefaultConfig()
	proxies := []string{}
	proxyManager := scrapers.NewProxyManager(proxies, 5*time.Minute)
	
	return &ScraperService{
		ufcEventScraper: scrapers.NewUFCEventScraper(config),
		proxyManager:    proxyManager,
	}
}

// convertUFCEventToModel converts a UFCEvent to a models.Event
func convertUFCEventToModel(ufcEvent *scrapers.UFCEvent) *models.Event {
	return &models.Event{
		Name:       ufcEvent.Name,
		Date:       ufcEvent.Date,
		Location:   fmt.Sprintf("%s, %s", ufcEvent.Venue, ufcEvent.Location),
		Promotion:  "UFC",
		MainCard:   []*models.Fight{},  // To be populated separately if needed
		PrelimCard: []*models.Fight{}, // To be populated separately if needed
		UFCUrl:     ufcEvent.UFCURL,    // Store the UFC URL
		EventType:  ufcEvent.EventType,
		Status:     ufcEvent.Status,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// ScrapeEvent scrapes a single event by URL
func (s *ScraperService) ScrapeEvent(ctx context.Context, url string) (*models.Event, error) {
	// For single event, we'd need to implement a method to scrape just one event page
	// This could be added to the UFCEventScraper
	return nil, fmt.Errorf("single event scraping not implemented yet for UFC source")
}

// ScrapeUpcomingEvents scrapes upcoming UFC events
func (s *ScraperService) ScrapeUpcomingEvents(ctx context.Context) ([]*models.Event, error) {
	// Only get the first page which contains upcoming events
	ufcEvents, err := s.ufcEventScraper.ScrapeEvents(ctx, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape upcoming events: %w", err)
	}
	
	// Only keep upcoming events
	var upcomingEvents []*scrapers.UFCEvent
	for _, event := range ufcEvents {
		if event.Status == "Scheduled" {
			upcomingEvents = append(upcomingEvents, event)
		}
	}
	
	// Convert UFCEvents to models.Events
	var events []*models.Event
	for _, ufcEvent := range upcomingEvents {
		event := convertUFCEventToModel(ufcEvent)
		events = append(events, event)
	}
	
	return events, nil
}

// ScrapeAllEvents scrapes all UFC events (both upcoming and past)
func (s *ScraperService) ScrapeAllEvents(ctx context.Context, maxPages int) ([]*models.Event, error) {
	ufcEvents, err := s.ufcEventScraper.ScrapeEvents(ctx, maxPages)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape all events: %w", err)
	}
	
	// Convert UFCEvents to models.Events
	var events []*models.Event
	for _, ufcEvent := range ufcEvents {
		event := convertUFCEventToModel(ufcEvent)
		events = append(events, event)
	}
	
	return events, nil
}

// UpdateProxies updates the proxy list
func (s *ScraperService) UpdateProxies(proxies []string) {
	s.proxyManager = scrapers.NewProxyManager(proxies, 5*time.Minute)
}

// GetNextProxy gets the next available proxy
func (s *ScraperService) GetNextProxy() (string, error) {
	return s.proxyManager.GetProxy()
}