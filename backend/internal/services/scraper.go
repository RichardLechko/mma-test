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

func (s *ScraperService) ScrapeEvent(ctx context.Context, url string) (*models.Event, error) {
	event, err := s.eventScraper.ScrapeEvent(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape event: %w", err)
	}
	return event, nil
}

func (s *ScraperService) ScrapeUpcomingEvents(ctx context.Context) ([]models.Event, error) {
	events, err := s.eventScraper.ScrapeUpcomingEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape upcoming events: %w", err)
	}
	return events, nil
}

func (s *ScraperService) UpdateProxies(proxies []string) {
	s.proxyManager = scrapers.NewProxyManager(proxies, 5*time.Minute)
}

func (s *ScraperService) GetNextProxy() (string, error) {
	return s.proxyManager.GetProxy()
}