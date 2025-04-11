package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"mma-scheduler/internal/models"
)

// EventService provides event-related operations
type EventService struct {
	db *sql.DB
}

// NewEventService creates a new event service
func NewEventService(db *sql.DB) EventServiceInterface {
	return &EventService{db: db}
}

// GetUpcomingEvents retrieves upcoming events from the database
func (s *EventService) GetUpcomingEvents(ctx context.Context) ([]*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, 
		       COALESCE(country, ''), COALESCE(city, ''), 
		       COALESCE(status, ''), 
		       COALESCE(wiki_url, ''), COALESCE(ufc_url, ''), 
		       created_at, updated_at 
		FROM events 
		WHERE event_date >= NOW() 
		ORDER BY event_date ASC 
		LIMIT 50
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query upcoming events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// GetEventsSince retrieves events since a specific date
func (s *EventService) GetEventsSince(ctx context.Context, since time.Time) ([]*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, 
		       COALESCE(country, ''), COALESCE(city, ''), 
		       COALESCE(status, ''), 
		       COALESCE(wiki_url, ''), COALESCE(ufc_url, ''), 
		       created_at, updated_at 
		FROM events 
		WHERE event_date >= $1 
		ORDER BY event_date ASC
	`

	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("query events since %v: %w", since, err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// GetEventByID retrieves an event by its ID
func (s *EventService) GetEventByID(ctx context.Context, id string) (*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, 
		       COALESCE(country, ''), COALESCE(city, ''), 
		       COALESCE(status, ''), 
		       COALESCE(wiki_url, ''), COALESCE(ufc_url, ''), 
		       created_at, updated_at 
		FROM events 
		WHERE id = $1
	`

	var event models.Event
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Name,
		&event.Date,
		&event.Venue,
		&event.Country,
		&event.City,
		&event.Status,
		&event.WikiURL,
		&event.UFCURL,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query event by ID: %w", err)
	}

	return &event, nil
}

// CreateEvent creates a new event in the database
func (s *EventService) CreateEvent(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO events (
			name, event_date, venue, country, city, 
			status, wiki_url, ufc_url, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		ON CONFLICT (name) DO UPDATE SET
			event_date = EXCLUDED.event_date,
			venue = EXCLUDED.venue,
			country = EXCLUDED.country,
			city = EXCLUDED.city,
			status = EXCLUDED.status,
			wiki_url = EXCLUDED.wiki_url,
			ufc_url = EXCLUDED.ufc_url,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	now := time.Now()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}

	err := s.db.QueryRowContext(ctx, query,
		event.Name,
		event.Date,
		event.Venue,
		event.Country,
		event.City,
		event.Status,
		event.WikiURL,
		event.UFCURL,
		event.CreatedAt,
		event.UpdatedAt,
	).Scan(&event.ID)

	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	return nil
}

// UpdateEvent updates an existing event
func (s *EventService) UpdateEvent(ctx context.Context, event *models.Event) error {
	query := `
		UPDATE events SET
			name = $1,
			event_date = $2,
			venue = $4,
			country = $5,
			city = $6,
			status = $7,
			wiki_url = $8,
			ufc_url = $9,
			updated_at = $10
		WHERE id = $11
	`

	now := time.Now()
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}

	result, err := s.db.ExecContext(ctx, query,
		event.Name,
		event.Date,
		event.Venue,
		event.Country,
		event.City,
		event.Status,
		event.WikiURL,
		event.UFCURL,
		event.UpdatedAt,
		event.ID,
	)

	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("event not found: %s", event.ID)
	}

	return nil
}

// DeleteEvent deletes an event by its ID
func (s *EventService) DeleteEvent(ctx context.Context, id string) error {
	query := `DELETE FROM events WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("event not found: %s", id)
	}

	return nil
}

// scanEvents is a helper function to scan rows into event structs
func (s *EventService) scanEvents(rows *sql.Rows) ([]*models.Event, error) {
	var events []*models.Event

	for rows.Next() {
		var event models.Event
		err := rows.Scan(
			&event.ID,
			&event.Name,
			&event.Date,
			&event.Venue,
			&event.Country,
			&event.City,
			&event.Status,
			&event.WikiURL,
			&event.UFCURL,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event row: %w", err)
		}
		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}