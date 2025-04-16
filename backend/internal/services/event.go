package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"mma-scheduler/internal/models"
)

// EventService handles database operations for events
type EventService struct {
	db *sql.DB
}

// NewEventService creates a new event service instance
func NewEventService(db *sql.DB) *EventService {
	return &EventService{
		db: db,
	}
}

// GetUpcomingEvents retrieves all events that are scheduled to occur in the future
func (s *EventService) GetUpcomingEvents(ctx context.Context) ([]*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, city, country, ufc_url, status, created_at, updated_at, attendance
		FROM events
		WHERE event_date >= CURRENT_DATE
		ORDER BY event_date ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query upcoming events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// GetEventsSince retrieves all events that occurred since the given date
func (s *EventService) GetEventsSince(ctx context.Context, since time.Time) ([]*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, city, country, ufc_url, status, created_at, updated_at, attendance
		FROM events
		WHERE event_date >= $1
		ORDER BY event_date ASC
	`

	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query events since %v: %w", since, err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// GetEventByID retrieves an event by its ID
func (s *EventService) GetEventByID(ctx context.Context, id string) (*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, city, country, ufc_url, status, created_at, updated_at, attendance
		FROM events
		WHERE id = $1
	`

	row := s.db.QueryRowContext(ctx, query, id)
	
	event := &models.Event{}
	var createdAt, updatedAt time.Time
	
	err := row.Scan(
		&event.ID,
		&event.Name,
		&event.Date,
		&event.Venue,
		&event.City,
		&event.Country,
		&event.UFCURL,
		&event.Status,
		&createdAt,
		&updatedAt,
		&event.Attendance,
	)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("event with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to scan event: %w", err)
	}
	
	return event, nil
}

// GetEventByUFCURL retrieves an event by its UFC URL
func (s *EventService) GetEventByUFCURL(ctx context.Context, ufcURL string) (*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, city, country, ufc_url, status, created_at, updated_at, attendance
		FROM events
		WHERE ufc_url = $1
	`

	row := s.db.QueryRowContext(ctx, query, ufcURL)
	
	event := &models.Event{}
	var createdAt, updatedAt time.Time
	
	err := row.Scan(
		&event.ID,
		&event.Name,
		&event.Date,
		&event.Venue,
		&event.City,
		&event.Country,
		&event.UFCURL,
		&event.Status,
		&createdAt,
		&updatedAt,
		&event.Attendance,
	)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil, nil if not found (to handle upsert logic)
		}
		return nil, fmt.Errorf("failed to scan event: %w", err)
	}
	
	return event, nil
}

// CreateEvent inserts a new event into the database
func (s *EventService) CreateEvent(ctx context.Context, event *models.Event) error {
	// First check if the event already exists by UFC URL
	existingEvent, err := s.GetEventByUFCURL(ctx, event.UFCURL)
	if err != nil {
		return fmt.Errorf("error checking for existing event: %w", err)
	}
	
	// If the event already exists, update it instead
	if existingEvent != nil {
		event.ID = existingEvent.ID // Ensure we update the correct record
		return s.UpdateEvent(ctx, event)
	}
	
	// Otherwise, insert a new record
	query := `
		INSERT INTO events (name, event_date, venue, city, country, ufc_url, status, attendance)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	row := s.db.QueryRowContext(
		ctx,
		query,
		event.Name,
		event.Date,
		event.Venue,
		event.City,
		event.Country,
		event.UFCURL,
		event.Status,
		event.Attendance,
	)

	// Get the ID of the newly inserted event
	err = row.Scan(&event.ID)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	log.Printf("Created event %s: %s at %s on %s", event.ID, event.Name, event.Venue, event.Date.Format("2006-01-02"))
	return nil
}

// UpdateEvent updates an existing event in the database
func (s *EventService) UpdateEvent(ctx context.Context, event *models.Event) error {
	query := `
		UPDATE events
		SET name = $1, event_date = $2, venue = $3, city = $4, country = $5, ufc_url = $6, status = $7, updated_at = CURRENT_TIMESTAMP, attendance = $8
		WHERE id = $8
	`

	result, err := s.db.ExecContext(
		ctx,
		query,
		event.Name,
		event.Date,
		event.Venue,
		event.City,
		event.Country,
		event.UFCURL,
		event.Status,
		event.ID,
		event.Attendance,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("event with ID %s not found", event.ID)
	}

	log.Printf("Updated event %s: %s", event.ID, event.Name)
	return nil
}

// DeleteEvent removes an event from the database
func (s *EventService) DeleteEvent(ctx context.Context, id string) error {
	query := `DELETE FROM events WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("event with ID %s not found", id)
	}

	log.Printf("Deleted event %s", id)
	return nil
}

// scanEvents is a helper function to scan multiple event rows
func (s *EventService) scanEvents(rows *sql.Rows) ([]*models.Event, error) {
	var events []*models.Event
	
	for rows.Next() {
		event := &models.Event{}
		var createdAt, updatedAt time.Time
		
		err := rows.Scan(
			&event.ID,
			&event.Name,
			&event.Date,
			&event.Venue,
			&event.City,
			&event.Country,
			&event.UFCURL,
			&event.Status,
			&createdAt,
			&updatedAt,
			&event.Attendance,
		)
		
		if err != nil {
			return nil, fmt.Errorf("failed to scan event row: %w", err)
		}
		
		events = append(events, event)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event rows: %w", err)
	}
	
	return events, nil
}