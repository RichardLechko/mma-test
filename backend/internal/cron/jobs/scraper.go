package jobs

import (
    "context"
    "fmt"
    "log"
    "time"

    "mma-scheduler/internal/models"
    "mma-scheduler/internal/services"
)

type ScraperJob struct {
    logger         *log.Logger
    scraperService services.ScraperServiceInterface
    eventService   services.EventServiceInterface
}

type ScraperConfig struct {
    MaxConcurrentScrapers int
    ScrapeDelay           time.Duration
    RetryDelay           time.Duration
    MaxRetries           int
}

func NewScraperJob(
    logger *log.Logger,
    scraperService services.ScraperServiceInterface,
    eventService services.EventServiceInterface,
) *ScraperJob {
    return &ScraperJob{
        logger:         logger,
        scraperService: scraperService,
        eventService:   eventService,
    }
}

func (j *ScraperJob) RunScraper(ctx context.Context) error {
    j.logger.Println("Starting scraping job")
    
    events, err := j.scraperService.ScrapeUpcomingEvents(ctx)
    if err != nil {
        return fmt.Errorf("failed to scrape events: %w", err)
    }

    for _, event := range events {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := j.validateScrapedData(&event); err != nil {
                j.logger.Printf("Invalid event data for %s: %v", event.Name, err)
                continue
            }

            if err := j.eventService.CreateEvent(ctx, &event); err != nil {
                j.logger.Printf("Error saving event %s: %v", event.Name, err)
                continue
            }
            
            time.Sleep(time.Second * 2) 
        }
    }

    j.logger.Println("Scraping job completed successfully")
    return nil
}

func (j *ScraperJob) validateScrapedData(event *models.Event) error {
    if event.Name == "" {
        return fmt.Errorf("event name cannot be empty")
    }
    if event.Date.IsZero() {
        return fmt.Errorf("event date cannot be empty")
    }
    if event.Location == "" {
        return fmt.Errorf("event location cannot be empty")
    }
    return nil
}