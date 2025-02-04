package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mma-scheduler/internal/services"
)

type CleanupJob struct {
	logger         *log.Logger
	fighterService services.FighterServiceInterface
	fightService   services.FightServiceInterface
	eventService   services.EventServiceInterface
	mediaService   *services.MediaService
}

type CleanupConfig struct {
	MaxMediaAge     time.Duration
	MaxLogAge       time.Duration
	BatchSize       int
	RetentionPeriod time.Duration
}

func NewCleanupJob(
	logger *log.Logger,
	fighterService services.FighterServiceInterface,
	fightService services.FightServiceInterface,
	eventService services.EventServiceInterface,
	mediaService *services.MediaService,
) *CleanupJob {
	return &CleanupJob{
		logger:         logger,
		fighterService: fighterService,
		fightService:   fightService,
		eventService:   eventService,
		mediaService:   mediaService,
	}
}

func (j *CleanupJob) PerformCleanup(ctx context.Context) error {
	j.logger.Println("Starting cleanup job")

	config := &CleanupConfig{
		MaxMediaAge:     90 * 24 * time.Hour,
		MaxLogAge:       30 * 24 * time.Hour,
		BatchSize:       100,
		RetentionPeriod: 365 * 24 * time.Hour,
	}

	errChan := make(chan error, 5)
	var wg sync.WaitGroup

	wg.Add(5)
	go j.cleanupOldMedia(ctx, config, &wg, errChan)
	go j.cleanupDuplicateRecords(ctx, &wg, errChan)
	go j.cleanupOrphanedRecords(ctx, &wg, errChan)
	go j.validateDataIntegrity(ctx, &wg, errChan)
	go j.archiveOldFights(ctx, config, &wg, errChan)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("cleanup job cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("cleanup error: %w", err)
		}
	case <-done:
		j.logger.Println("Cleanup job completed successfully")
	}

	return nil
}

func (j *CleanupJob) cleanupOldMedia(ctx context.Context, config *CleanupConfig, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	cutoffDate := time.Now().Add(-config.MaxMediaAge)
	j.logger.Printf("Cleaning up media files older than %v", cutoffDate)

	offset := 0
	for {
		media, err := j.mediaService.GetMediaBeforeDate(ctx, cutoffDate, config.BatchSize, offset)
		if err != nil {
			errChan <- fmt.Errorf("failed to get old media: %w", err)
			return
		}

		if len(media) == 0 {
			break
		}

		for _, m := range media {
			if err := j.mediaService.DeleteMedia(ctx, m.ID); err != nil {
				j.logger.Printf("Error deleting media %s: %v", m.ID, err)
				continue
			}
		}

		offset += config.BatchSize
	}
}

func (j *CleanupJob) cleanupDuplicateRecords(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	fighters, err := j.fighterService.GetDuplicateFighters(ctx)
	if err != nil {
		errChan <- fmt.Errorf("failed to get duplicate fighters: %w", err)
		return
	}

	for _, group := range fighters {
		if len(group) > 1 {
			mainFighter := group[0]
			duplicates := group[1:]

			if err := j.fighterService.MergeFighters(ctx, mainFighter.ID, duplicates); err != nil {
				j.logger.Printf("Error merging fighters: %v", err)
			}
		}
	}

	events, err := j.eventService.GetDuplicateEvents(ctx)
	if err != nil {
		errChan <- fmt.Errorf("failed to get duplicate events: %w", err)
		return
	}

	for _, group := range events {
		if len(group) > 1 {
			for _, dup := range group[1:] {
				if err := j.eventService.DeleteEvent(ctx, dup.ID); err != nil {
					j.logger.Printf("Error deleting duplicate event: %v", err)
				}
			}
		}
	}
}

func (j *CleanupJob) validateDataIntegrity(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	if err := j.fighterService.ValidateRecords(ctx); err != nil {
		errChan <- fmt.Errorf("failed to validate fighter records: %w", err)
		return
	}

	if err := j.eventService.ValidateEventData(ctx); err != nil {
		errChan <- fmt.Errorf("failed to validate event data: %w", err)
		return
	}

	if err := j.fightService.ValidateFightResults(ctx); err != nil {
		errChan <- fmt.Errorf("failed to validate fight results: %w", err)
		return
	}
}

func (j *CleanupJob) archiveOldFights(ctx context.Context, config *CleanupConfig, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	cutoffDate := time.Now().Add(-config.RetentionPeriod)

	if err := j.fightService.ArchiveOldFights(ctx, cutoffDate); err != nil {
		errChan <- fmt.Errorf("failed to archive old fights: %w", err)
		return
	}
}

func (j *CleanupJob) cleanupOrphanedRecords(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
	defer wg.Done()

	if err := j.mediaService.DeleteOrphanedMedia(ctx); err != nil {
		errChan <- fmt.Errorf("failed to cleanup orphaned media: %w", err)
		return
	}
}
