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
            id, name, event_date, location, promotion, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7
        ) RETURNING id`

    id := uuid.New().String()
    now := time.Now()
    err := s.db.QueryRowContext(
        ctx,
        query,
        id,
        event.Name,
        event.Date,
        event.Location,
        event.Promotion,
        now,
        now,
    ).Scan(&event.ID)

    if err != nil {
        return err
    }

    return nil
}

func (s *EventService) GetEventByID(ctx context.Context, id string) (*models.Event, error) {
    query := `
        SELECT id, name, event_date, location, promotion, created_at, updated_at
        FROM events
        WHERE id = $1`

    event := &models.Event{}
    err := s.db.QueryRowContext(ctx, query, id).Scan(
        &event.ID,
        &event.Name,
        &event.Date,
        &event.Location,
        &event.Promotion,
        &event.CreatedAt,
        &event.UpdatedAt,
    )

    if err == sql.ErrNoRows {
        return nil, errors.New("event not found")
    }
    if err != nil {
        return nil, err
    }

    mainCard, err := s.getMainCardFights(ctx, id)
    if err != nil {
        return nil, err
    }
    event.MainCard = mainCard

    prelimCard, err := s.getPrelimCardFights(ctx, id)
    if err != nil {
        return nil, err
    }
    event.PrelimCard = prelimCard

    return event, nil
}

func (s *EventService) ListEvents(ctx context.Context, filters map[string]interface{}) ([]models.Event, error) {
    query := `
        SELECT id, name, event_date, location, promotion, created_at, updated_at
        FROM events
        WHERE ($1::timestamp IS NULL OR event_date >= $1)
        ORDER BY event_date DESC`

    fromDate := filters["from_date"]

    rows, err := s.db.QueryContext(ctx, query, fromDate)
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
            &event.Location,
            &event.Promotion,
            &event.CreatedAt,
            &event.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }

        mainCard, err := s.getMainCardFights(ctx, event.ID)
        if err != nil {
            return nil, err
        }
        event.MainCard = mainCard

        prelimCard, err := s.getPrelimCardFights(ctx, event.ID)
        if err != nil {
            return nil, err
        }
        event.PrelimCard = prelimCard

        events = append(events, event)
    }

    return events, nil
}

func (s *EventService) UpdateEvent(ctx context.Context, event *models.Event) error {
    query := `
        UPDATE events
        SET name = $1, event_date = $2, location = $3, promotion = $4, updated_at = $5
        WHERE id = $6`

    result, err := s.db.ExecContext(
        ctx,
        query,
        event.Name,
        event.Date,
        event.Location,
        event.Promotion,
        time.Now(),
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
        SELECT id, name, event_date, location, promotion, created_at, updated_at
        FROM events
        WHERE event_date > NOW()
        ORDER BY event_date ASC
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
            &e.Location,
            &e.Promotion,
            &e.CreatedAt,
            &e.UpdatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("scan event: %w", err)
        }
        events = append(events, &e)
    }

    return events, nil
}

func (s *EventService) getMainCardFights(ctx context.Context, eventID string) ([]models.Fight, error) {
    query := `
        SELECT id, event_id, fighter_1, fighter_2, weight_class, is_main_event, fight_order 
        FROM fights 
        WHERE event_id = $1 AND is_main_card = true
        ORDER BY fight_order ASC`

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
            &fight.EventID,
            &fight.Fighter1,
            &fight.Fighter2,
            &fight.WeightClass,
            &fight.IsMainEvent,
            &fight.Order,
        )
        if err != nil {
            return nil, err
        }
        fights = append(fights, fight)
    }

    return fights, nil
}

func (s *EventService) getPrelimCardFights(ctx context.Context, eventID string) ([]models.Fight, error) {
    query := `
        SELECT id, event_id, fighter_1, fighter_2, weight_class, is_main_event, fight_order 
        FROM fights 
        WHERE event_id = $1 AND is_main_card = false
        ORDER BY fight_order ASC`

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
            &fight.EventID,
            &fight.Fighter1,
            &fight.Fighter2,
            &fight.WeightClass,
            &fight.IsMainEvent,
            &fight.Order,
        )
        if err != nil {
            return nil, err
        }
        fights = append(fights, fight)
    }

    return fights, nil
}

func (s *EventService) GetEventsSince(ctx context.Context, since time.Time) ([]*models.Event, error) {
    query := `
        SELECT id, name, event_date, location, promotion, created_at, updated_at
        FROM events
        WHERE event_date >= $1
        ORDER BY event_date DESC
    `

    rows, err := s.db.QueryContext(ctx, query, since)
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
            &event.Date,
            &event.Location,
            &event.Promotion,
            &event.CreatedAt,
            &event.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        events = append(events, event)
    }

    return events, nil
}