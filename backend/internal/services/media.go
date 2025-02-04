package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mma-scheduler/internal/models"
	"time"
)

type MediaService struct {
	db *sql.DB
}

func NewMediaService(db *sql.DB) *MediaService {
	return &MediaService{
		db: db,
	}
}

func (s *MediaService) GetOldMedia(ctx context.Context, cutoffDate time.Time, limit, offset int) ([]*models.Media, error) {
	query := `
		SELECT m.id, m.fight_id, m.media_type, m.url, m.storage_location
		FROM fight_media m
		JOIN fights f ON m.fight_id = f.id
		WHERE f.fight_date < $1 AND m.is_archived = false
		ORDER BY f.fight_date
		LIMIT $2 OFFSET $3
	`

	rows, err := s.db.QueryContext(ctx, query, cutoffDate, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaFiles []*models.Media
	for rows.Next() {
		media := &models.Media{}
		err := rows.Scan(
			&media.ID,
			&media.FightID,
			&media.MediaType,
			&media.URL,
			&media.StorageLocation,
		)
		if err != nil {
			return nil, err
		}
		mediaFiles = append(mediaFiles, media)
	}

	return mediaFiles, nil
}

func (s *MediaService) DeleteMedia(ctx context.Context, id string) error {
	query := `DELETE FROM fight_media WHERE id = $1`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("media not found")
	}

	return nil
}

func (s *MediaService) GetMediaBeforeDate(ctx context.Context, cutoffDate time.Time, limit, offset int) ([]*models.Media, error) {
	query := `
        SELECT id, fight_id, media_type, url, storage_location, archive_location, is_archived
        FROM fight_media
        WHERE created_at < $1
        ORDER BY created_at
        LIMIT $2 OFFSET $3
    `

	rows, err := s.db.QueryContext(ctx, query, cutoffDate, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query old media: %w", err)
	}
	defer rows.Close()

	var media []*models.Media
	for rows.Next() {
		var m models.Media
		err := rows.Scan(
			&m.ID,
			&m.FightID,
			&m.MediaType,
			&m.URL,
			&m.StorageLocation,
			&m.ArchiveLocation,
			&m.IsArchived,
		)
		if err != nil {
			return nil, fmt.Errorf("scan media: %w", err)
		}
		media = append(media, &m)
	}

	return media, nil
}

func (s *MediaService) DeleteOrphanedMedia(ctx context.Context) error {
	query := `
        DELETE FROM fight_media
        WHERE fight_id NOT IN (SELECT id FROM fights)
    `

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("delete orphaned media: %w", err)
	}

	return nil
}
