package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"mma-scheduler/internal/models"
	"mma-scheduler/internal/services"
)

type ArchiveJob struct {
    logger          *log.Logger
    db              *sql.DB                    
    fightService    *services.FightService
    eventService    *services.EventService     
    mediaService    *services.MediaService     
}

type ArchiveConfig struct {
	ArchiveAge        time.Duration
	BatchSize         int
	EnableCompression bool
	CompressionLevel  int
	RetentionPolicy   string
}

func NewArchiveJob(
    logger *log.Logger,
    db *sql.DB,
    fightService *services.FightService,
    eventService *services.EventService,
    mediaService *services.MediaService,
) *ArchiveJob {
    return &ArchiveJob{
        logger:          logger,
        db:              db,
        fightService:    fightService,
        eventService:    eventService,
        mediaService:    mediaService,
    }
}

func (j *ArchiveJob) ArchiveOldFights(ctx context.Context) error {
    j.logger.Println("Starting archival job")

    config := &ArchiveConfig{
        ArchiveAge:        365 * 24 * time.Hour,
        BatchSize:         100,
        EnableCompression: true,
        CompressionLevel:  6,
        RetentionPolicy:   "yearly",
    }

    tx, err := j.db.BeginTx(ctx, &sql.TxOptions{})
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    errChan := make(chan error, 3)
    var wg sync.WaitGroup

    wg.Add(3)
    go j.archiveFights(ctx, tx, config, &wg, errChan)
    go j.archiveMedia(ctx, tx, config, &wg, errChan)
    go j.updatePartitions(ctx, tx, &wg, errChan)

    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-ctx.Done():
        return fmt.Errorf("archive job cancelled: %w", ctx.Err())
    case err := <-errChan:
        if err != nil {
            return fmt.Errorf("archive error: %w", err)
        }
    case <-done:
        j.logger.Println("Archive job completed successfully")
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    if err := j.verifyArchivedData(ctx); err != nil {
        return fmt.Errorf("archive verification failed: %w", err)
    }

    return nil
}

func (j *ArchiveJob) archiveFights(ctx context.Context, tx *sql.Tx, config *ArchiveConfig, wg *sync.WaitGroup, errChan chan error) {
    defer wg.Done()

    cutoffDate := time.Now().Add(-config.ArchiveAge)
    j.logger.Printf("Archiving fights older than %v", cutoffDate)

    offset := 0
    for {
        fights, err := j.fightService.GetOldFights(ctx, cutoffDate, config.BatchSize, offset)
        if err != nil {
            errChan <- fmt.Errorf("failed to get old fights: %w", err)
            return
        }

        if len(fights) == 0 {
            break
        }

        for _, fight := range fights {
            if err := j.moveToHistoricalPartition(ctx, tx, fight); err != nil {
                j.logger.Printf("Error archiving fight %s: %v", fight.ID, err)
                continue
            }
        }

        offset += config.BatchSize
    }
}

func (j *ArchiveJob) archiveMedia(ctx context.Context, tx *sql.Tx, config *ArchiveConfig, wg *sync.WaitGroup, errChan chan error) {
    defer wg.Done()

    cutoffDate := time.Now().Add(-config.ArchiveAge)
    j.logger.Printf("Archiving media older than %v", cutoffDate)

    offset := 0
    for {
        mediaFiles, err := j.mediaService.GetOldMedia(ctx, cutoffDate, config.BatchSize, offset)
        if err != nil {
            errChan <- fmt.Errorf("failed to get old media: %w", err)
            return
        }

        if len(mediaFiles) == 0 {
            break
        }

        for _, media := range mediaFiles {
            if err := j.archiveMediaFile(ctx, tx, media, config); err != nil {
                j.logger.Printf("Error archiving media %s: %v", media.ID, err)
                continue
            }
        }

        offset += config.BatchSize
    }
}

func (j *ArchiveJob) updatePartitions(ctx context.Context, tx *sql.Tx, wg *sync.WaitGroup, errChan chan error) {
    defer wg.Done()

    if _, err := tx.ExecContext(ctx, "SELECT maintain_fight_partitions()"); err != nil {
        errChan <- fmt.Errorf("failed to maintain partitions: %w", err)
        return
    }
}

func (j *ArchiveJob) moveToHistoricalPartition(ctx context.Context, tx *sql.Tx, fight *models.Fight) error {
    query := `
        INSERT INTO historical_fights
        SELECT * FROM fights WHERE id = $1
    `
    if _, err := tx.ExecContext(ctx, query, fight.ID); err != nil {
        return fmt.Errorf("failed to insert into historical partition: %w", err)
    }

    query = `DELETE FROM fights WHERE id = $1`
    if _, err := tx.ExecContext(ctx, query, fight.ID); err != nil {
        return fmt.Errorf("failed to delete from fights: %w", err)
    }

    return nil
}

func (j *ArchiveJob) archiveMediaFile(ctx context.Context, tx *sql.Tx, media *models.Media, config *ArchiveConfig) error {
    if config.EnableCompression {
        if err := j.compressMedia(media, config.CompressionLevel); err != nil {
            return fmt.Errorf("failed to compress media: %w", err)
        }
    }

    if err := j.moveToArchiveStorage(ctx, media); err != nil {
        return fmt.Errorf("failed to move to archive storage: %w", err)
    }

    query := `
        UPDATE fight_media
        SET storage_location = $1, is_archived = true
        WHERE id = $2
    `
    if _, err := tx.ExecContext(ctx, query, media.ArchiveLocation, media.ID); err != nil {
        return fmt.Errorf("failed to update media record: %w", err)
    }

    return nil
}

func (j *ArchiveJob) compressMedia(media *models.Media, level int) error {
    return nil
}

func (j *ArchiveJob) moveToArchiveStorage(ctx context.Context, media *models.Media) error {
    return nil
}

func (j *ArchiveJob) verifyArchivedData(ctx context.Context) error {
    query := `
        SELECT COUNT(*) 
        FROM historical_fights 
        WHERE fight_date IS NULL 
           OR fight_details IS NULL
    `
    var invalidCount int
    if err := j.db.QueryRowContext(ctx, query).Scan(&invalidCount); err != nil {
        return fmt.Errorf("failed to verify historical fights: %w", err)
    }

    if invalidCount > 0 {
        return fmt.Errorf("found %d invalid historical fight records", invalidCount)
    }

    query = `
        SELECT COUNT(*) 
        FROM fight_media 
        WHERE is_archived = true 
          AND storage_location IS NULL
    `
    if err := j.db.QueryRowContext(ctx, query).Scan(&invalidCount); err != nil {
        return fmt.Errorf("failed to verify archived media: %w", err)
    }

    if invalidCount > 0 {
        return fmt.Errorf("found %d invalid archived media records", invalidCount)
    }

    return nil
}