package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mma-scheduler/internal/models"

	"github.com/google/uuid"
)

type EventService struct {
	db *sql.DB
}

var _ EventServiceInterface = (*EventService)(nil)

func NewEventService(db *sql.DB) *EventService {
	return &EventService{
		db: db,
	}
}

func (s *EventService) CreateEvent(ctx context.Context, event *models.Event) error {
	query := `
		INSERT INTO events (
			id, name, event_date, venue, city, country, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING id`

	id := uuid.New().String()
	err := s.db.QueryRowContext(
		ctx,
		query,
		id,
		event.Name,
		event.Date,
		event.Venue,
		event.City,
		event.Country,
		event.Status,
	).Scan(&event.ID)

	if err != nil {
		return err
	}

	return nil
}

func (s *EventService) GetEventByID(ctx context.Context, id string) (*models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, city, country, status
		FROM events
		WHERE id = $1`

	event := &models.Event{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&event.ID,
		&event.Name,
		&event.Date,
		&event.Venue,
		&event.City,
		&event.Country,
		&event.Status,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("event not found")
	}
	if err != nil {
		return nil, err
	}

	fights, err := s.getFightsForEvent(ctx, id)
	if err != nil {
		return nil, err
	}
	event.Fights = fights

	return event, nil
}

func (s *EventService) ListEvents(ctx context.Context, filters map[string]interface{}) ([]models.Event, error) {
	query := `
		SELECT id, name, event_date, venue, city, country, status
		FROM events
		WHERE ($1::text IS NULL OR status = $1)
		AND ($2::timestamp IS NULL OR event_date >= $2)
		ORDER BY event_date DESC`

	status := filters["status"]
	fromDate := filters["from_date"]

	rows, err := s.db.QueryContext(ctx, query, status, fromDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		err := rows.Scan(
			&event.ID,
			&event.Name,
			&event.Date,
			&event.Venue,
			&event.City,
			&event.Country,
			&event.Status,
		)
		if err != nil {
			return nil, err
		}

		fights, err := s.getFightsForEvent(ctx, event.ID)
		if err != nil {
			return nil, err
		}
		event.Fights = fights

		events = append(events, event)
	}

	return events, nil
}

func (s *EventService) UpdateEvent(ctx context.Context, event *models.Event) error {
	query := `
		UPDATE events
		SET name = $1, event_date = $2, venue = $3, city = $4, country = $5, status = $6
		WHERE id = $7`

	result, err := s.db.ExecContext(
		ctx,
		query,
		event.Name,
		event.Date,
		event.Venue,
		event.City,
		event.Country,
		event.Status,
		event.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("event not found")
	}

	return nil
}

func (s *EventService) DeleteEvent(ctx context.Context, id string) error {
	query := `DELETE FROM events WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("event not found")
	}

	return nil
}

func (s *EventService) GetUpcomingEvents(ctx context.Context) ([]*models.Event, error) {
	query := `
        SELECT id, name, date, venue, city, country, status, attendance, ppv_buys
        FROM events
        WHERE date > NOW()
        ORDER BY date ASC
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query upcoming events: %w", err)
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		var e models.Event
		err := rows.Scan(
			&e.ID,
			&e.Name,
			&e.Date,
			&e.Venue,
			&e.City,
			&e.Country,
			&e.Status,
			&e.Attendance,
			&e.PPVBuys,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, &e)
	}

	return events, nil
}

func (s *EventService) getFightsForEvent(ctx context.Context, eventID string) ([]models.Fight, error) {
	query := `
        SELECT f.id, f.weight_class, f.is_title_fight, f.is_main_event,
               f.fighter1_id, f.fighter2_id
        FROM fights f
        WHERE f.event_id = $1
        ORDER BY f.fight_order DESC`

	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fights []models.Fight
	for rows.Next() {
		var fight models.Fight
		err := rows.Scan(
			&fight.ID,
			&fight.WeightClass,
			&fight.IsTitleFight,
			&fight.IsMainEvent,
			&fight.Fighter1ID,
			&fight.Fighter2ID,
		)
		if err != nil {
			return nil, err
		}
		fights = append(fights, fight)
	}

	return fights, nil
}

func (s *EventService) GetEventsSince(ctx context.Context, startDate time.Time) ([]*models.Event, error) {
	query := `
        SELECT id, name, venue, attendance, ppv_buys
        FROM events
        WHERE event_date >= $1
        ORDER BY event_date DESC
    `

	rows, err := s.db.QueryContext(ctx, query, startDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event := &models.Event{}
		err := rows.Scan(
			&event.ID,
			&event.Name,
			&event.Venue,
			&event.Attendance,
			&event.PPVBuys,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

func (s *EventService) GetDuplicateEvents(ctx context.Context) ([][]*models.Event, error) {
	query := `
        WITH duplicate_events AS (
            SELECT 
                array_agg(id) as ids,
                name,
                event_date,
                COUNT(*) as count
            FROM events
            GROUP BY name, event_date
            HAVING COUNT(*) > 1
        )
        SELECT e.id, e.name, e.date, e.venue, e.city, e.country, e.status, e.attendance, e.ppv_buys
        FROM events e
        WHERE e.id IN (
            SELECT unnest(ids)
            FROM duplicate_events
        )
        ORDER BY e.date, e.name, e.id
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query duplicate events: %w", err)
	}
	defer rows.Close()

	eventMap := make(map[string][]*models.Event)

	for rows.Next() {
		var e models.Event
		err := rows.Scan(
			&e.ID,
			&e.Name,
			&e.Date,
			&e.Venue,
			&e.City,
			&e.Country,
			&e.Status,
			&e.Attendance,
			&e.PPVBuys,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		key := fmt.Sprintf("%s_%s", e.Date.Format("2006-01-02"), e.Name)
		eventMap[key] = append(eventMap[key], &e)
	}

	var result [][]*models.Event
	for _, events := range eventMap {
		result = append(result, events)
	}

	return result, nil
}

func (s *EventService) ValidateEventData(ctx context.Context) error {

	query := `
        UPDATE events e
        SET fight_card_order = (
            SELECT jsonb_agg(f.id ORDER BY f.fight_order)
            FROM fights f
            WHERE f.event_id = e.id
        )
        WHERE jsonb_array_length(fight_card_order) != (
            SELECT COUNT(*)
            FROM fights f
            WHERE f.event_id = e.id
        )
    `

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("validate event fight cards: %w", err)
	}

	return nil
}

func (s *EventService) GetPromotionEventsSince(ctx context.Context, promotionID string, since time.Time) ([]*models.Event, error) {
	query := `
        SELECT id, name, date, venue, city, country, status, attendance, ppv_buys
        FROM events
        WHERE promotion_id = $1
        AND date >= $2
        ORDER BY date ASC
    `

	rows, err := s.db.QueryContext(ctx, query, promotionID, since)
	if err != nil {
		return nil, fmt.Errorf("query promotion events: %w", err)
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		var e models.Event
		err := rows.Scan(
			&e.ID,
			&e.Name,
			&e.Date,
			&e.Venue,
			&e.City,
			&e.Country,
			&e.Status,
			&e.Attendance,
			&e.PPVBuys,
		)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, &e)
	}

	return events, nil
}
