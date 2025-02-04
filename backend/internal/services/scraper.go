package services

import (
	"context"
	"fmt"
	"time"

	"mma-scheduler/internal/models"
	"mma-scheduler/pkg/scrapers"
)

type ScraperService struct {
	eventScraper   *scrapers.EventScraper
	fighterScraper *scrapers.FighterScraper
	processor      *ProcessorService
	proxyManager   *scrapers.ProxyManager
}

func NewScraperService(processor *ProcessorService) *ScraperService {
	config := scrapers.DefaultConfig()

	proxies := []string{}
	proxyManager := scrapers.NewProxyManager(proxies, 5*time.Minute)

	return &ScraperService{
		eventScraper:   scrapers.NewEventScraper(config),
		fighterScraper: scrapers.NewFighterScraper(config),
		processor:      processor,
		proxyManager:   proxyManager,
	}
}

func (s *ScraperService) ScrapeEvent(ctx context.Context, url string) (*models.Event, error) {
	event, err := s.eventScraper.ScrapeEvent(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape event: %w", err)
	}

	if err := s.processor.ProcessEventData(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to process event data: %w", err)
	}

	return event, nil
}

func (s *ScraperService) ScrapeUpcomingEvents(ctx context.Context) ([]models.Event, error) {
	events, err := s.eventScraper.ScrapeUpcomingEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape upcoming events: %w", err)
	}

	for i := range events {
		if err := s.processor.ProcessEventData(ctx, &events[i]); err != nil {
			return nil, fmt.Errorf("failed to process event %s: %w", events[i].Name, err)
		}
	}

	return events, nil
}

func (s *ScraperService) ScrapeFighter(ctx context.Context, url string) (*models.Fighter, error) {
	fighter, err := s.fighterScraper.ScrapeFighter(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape fighter: %w", err)
	}

	if err := s.processor.ProcessFighterData(ctx, fighter); err != nil {
		return nil, fmt.Errorf("failed to process fighter data: %w", err)
	}

	return fighter, nil
}

func (s *ScraperService) ScrapeRoster(ctx context.Context) ([]models.Fighter, error) {
	fighters, err := s.fighterScraper.ScrapeRoster(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape roster: %w", err)
	}

	for i := range fighters {
		if err := s.processor.ProcessFighterData(ctx, &fighters[i]); err != nil {
			return nil, fmt.Errorf("failed to process fighter %s: %w", fighters[i].FullName, err)
		}
	}

	return fighters, nil
}

func (s *ScraperService) ScrapeFights(ctx context.Context, eventURL string) ([]models.Fight, error) {
	event, err := s.eventScraper.ScrapeEvent(ctx, eventURL)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape event fights: %w", err)
	}

	for i := range event.Fights {
		if err := s.processor.ProcessFightData(ctx, &event.Fights[i]); err != nil {
			return nil, fmt.Errorf("failed to process fight data: %w", err)
		}
	}

	return event.Fights, nil
}

func (s *ScraperService) UpdateProxies(proxies []string) {
	s.proxyManager = scrapers.NewProxyManager(proxies, 5*time.Minute)
}

func (s *ScraperService) GetNextProxy() (string, error) {
	return s.proxyManager.GetProxy()
}

func (s *ScraperService) ScrapeFightResults(ctx context.Context, fightID string) (*models.FightResult, error) {

	result := &models.FightResult{
		WinnerID: "",
		Method:   "",
		Round:    0,
		Time:     "",
	}

	return result, nil
}
