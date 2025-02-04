package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mma-scheduler/internal/models"
	"mma-scheduler/internal/services"
)

type ScraperJob struct {
	logger         *log.Logger
	scraperService services.ScraperServiceInterface
	fighterService services.FighterServiceInterface
	eventService   services.EventServiceInterface
	fightService   services.FightServiceInterface
}

type ScraperConfig struct {
	MaxConcurrentScrapers int
	ScrapeDelay           time.Duration
	RetryDelay            time.Duration
	MaxRetries            int
}

func NewScraperJob(
	logger *log.Logger,
	scraperService services.ScraperServiceInterface,
	fighterService services.FighterServiceInterface,
	eventService services.EventServiceInterface,
	fightService services.FightServiceInterface,
) *ScraperJob {
	return &ScraperJob{
		logger:         logger,
		scraperService: scraperService,
		fighterService: fighterService,
		eventService:   eventService,
		fightService:   fightService,
	}
}

func (j *ScraperJob) RunScraper(ctx context.Context) error {
	j.logger.Println("Starting scraping job")

	errChan := make(chan error, 3)
	var wg sync.WaitGroup

	wg.Add(3)
	go j.scrapeFighters(ctx, &wg, errChan)
	go j.scrapeEvents(ctx, &wg, errChan)
	go j.scrapeFightResults(ctx, &wg, errChan)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("scraping job cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("scraping error: %w", err)
		}
	case <-done:
		j.logger.Println("Scraping job completed successfully")
	}

	return nil
}

func (j *ScraperJob) scrapeFighters(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	j.logger.Println("Starting fighter scraping")

	fighters, err := j.fighterService.GetFightersToUpdate(ctx)
	if err != nil {
		errChan <- fmt.Errorf("failed to get fighters to update: %w", err)
		return
	}

	workerPool := make(chan struct{}, 5)
	var scrapeWg sync.WaitGroup

	for _, fighter := range fighters {
		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		case workerPool <- struct{}{}:
			scrapeWg.Add(1)
			go func(fighterID string) {
				defer scrapeWg.Done()
				defer func() { <-workerPool }()

				fighterData, err := j.scraperService.ScrapeFighter(ctx, fighterID)
				if err != nil {
					j.logger.Printf("Error scraping fighter %s: %v", fighterID, err)
					return
				}

				if err := j.fighterService.UpdateFighter(ctx, fighterData); err != nil {
					j.logger.Printf("Error updating fighter %s: %v", fighterID, err)
					return
				}
			}(fighter.ID)
		}
	}

	scrapeWg.Wait()
}

func (j *ScraperJob) scrapeEvents(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	j.logger.Println("Starting event scraping")

	events, err := j.eventService.GetUpcomingEvents(ctx)
	if err != nil {
		errChan <- fmt.Errorf("failed to get upcoming events: %w", err)
		return
	}

	for _, event := range events {
		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		default:
			eventData, err := j.scraperService.ScrapeEvent(ctx, event.ID)
			if err != nil {
				j.logger.Printf("Error scraping event %s: %v", event.ID, err)
				continue
			}

			if err := j.eventService.UpdateEvent(ctx, eventData); err != nil {
				j.logger.Printf("Error updating event %s: %v", event.ID, err)
				continue
			}

			time.Sleep(time.Second * 2)
		}
	}
}

func (j *ScraperJob) scrapeFightResults(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	j.logger.Println("Starting fight result scraping")

	fights, err := j.fightService.GetFightsWithoutResults(ctx)
	if err != nil {
		errChan <- fmt.Errorf("failed to get fights without results: %w", err)
		return
	}

	for _, fight := range fights {
		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
			return
		default:
			results, err := j.scraperService.ScrapeFightResults(ctx, fight.ID)
			if err != nil {
				j.logger.Printf("Error scraping fight results %s: %v", fight.ID, err)
				continue
			}

			if err := j.fightService.UpdateFightResults(ctx, fight.ID, results); err != nil {
				j.logger.Printf("Error updating fight results %s: %v", fight.ID, err)
				continue
			}

			time.Sleep(time.Second * 2)
		}
	}
}

func (j *ScraperJob) validateScrapedData(data interface{}) error {
	// TODO: Implement validation logic based on data type
	switch v := data.(type) {
	case *models.Fighter:
		if v.FullName == "" {
			return fmt.Errorf("fighter name cannot be empty")
		}
	case *models.Event:
		if v.Name == "" {
			return fmt.Errorf("event name cannot be empty")
		}
	case *models.Fight:
		if v.Fighter1ID == "" || v.Fighter2ID == "" {
			return fmt.Errorf("fight must have two fighters")
		}
	}
	return nil
}

func (j *ScraperJob) handleScrapingError(ctx context.Context, task string, err error) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("task %s cancelled: %w", task, ctx.Err())
	default:
		j.logger.Printf("Error in %s: %v", task, err)
		return fmt.Errorf("task %s failed: %w", task, err)
	}
}

func (j *ScraperJob) updateLastScrapedTime(ctx context.Context, entityType, entityID string) error {
	// TODO: Implement timestamp update in database

	return nil
}
