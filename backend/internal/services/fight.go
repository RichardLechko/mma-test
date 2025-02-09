package services

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "mma-scheduler/internal/models"
    
    "github.com/google/uuid"
)

type FightService struct {
    db *sql.DB
}

func NewFightService(db *sql.DB) FightServiceInterface {
    return &FightService{
        db: db,
    }
}

func (s *FightService) CreateFight(ctx context.Context, fight *models.Fight) error {
    query := `
        INSERT INTO fights (
            id, event_id, fighter_1, fighter_2, weight_class,
            is_main_event, fight_order
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7
        ) RETURNING id`

    id := uuid.New().String()
    err := s.db.QueryRowContext(
        ctx,
        query,
        id,
        fight.EventID,
        fight.Fighter1,
        fight.Fighter2,
        fight.WeightClass,
        fight.IsMainEvent,
        fight.Order,
    ).Scan(&fight.ID)

    if err != nil {
        return err
    }

    return nil
}

func (s *FightService) GetFightByID(ctx context.Context, id string) (*models.Fight, error) {
    query := `
        SELECT 
            id, event_id, fighter_1, fighter_2, weight_class,
            is_main_event, fight_order,
            COALESCE(
                fight_details->>'winner_id',
                ''
            ) as winner_id,
            COALESCE(
                fight_details->>'finish_type',
                ''
            ) as method,
            COALESCE(
                (fight_details->>'rounds_completed')::int,
                0
            ) as round,
            COALESCE(
                fight_details->>'round_time',
                ''
            ) as time,
            COALESCE(
                fight_details->>'finish_details',
                ''
            ) as description
        FROM fights
        WHERE id = $1`

    fight := &models.Fight{}
    result := &models.FightResult{}

    err := s.db.QueryRowContext(ctx, query, id).Scan(
        &fight.ID,
        &fight.EventID,
        &fight.Fighter1,
        &fight.Fighter2,
        &fight.WeightClass,
        &fight.IsMainEvent,
        &fight.Order,
        &result.WinnerID,
        &result.Method,
        &result.Round,
        &result.Time,
        &result.Description,
    )

    if err == sql.ErrNoRows {
        return nil, errors.New("fight not found")
    }
    if err != nil {
        return nil, err
    }

    return fight, nil
}

func (s *FightService) ListFights(ctx context.Context, filters map[string]interface{}) ([]models.Fight, error) {
    query := `
        SELECT 
            id, event_id, fighter_1, fighter_2, weight_class,
            is_main_event, fight_order
        FROM fights
        WHERE ($1::text IS NULL OR event_id = $1)
        AND ($2::text IS NULL OR fighter_1 = $2 OR fighter_2 = $2)
        AND ($3::text IS NULL OR weight_class = $3)
        ORDER BY fight_order DESC`

    rows, err := s.db.QueryContext(ctx, query,
        filters["event_id"],
        filters["fighter_id"],
        filters["weight_class"],
    )
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

func (s *FightService) UpdateFight(ctx context.Context, fight *models.Fight) error {
    query := `
        UPDATE fights
        SET 
            event_id = $1,
            fighter_1 = $2,
            fighter_2 = $3,
            weight_class = $4,
            is_main_event = $5,
            fight_order = $6
        WHERE id = $7`

    result, err := s.db.ExecContext(
        ctx,
        query,
        fight.EventID,
        fight.Fighter1,
        fight.Fighter2,
        fight.WeightClass,
        fight.IsMainEvent,
        fight.Order,
        fight.ID,
    )
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }
    if rows == 0 {
        return errors.New("fight not found")
    }

    return nil
}

func (s *FightService) DeleteFight(ctx context.Context, id string) error {
    query := `DELETE FROM fights WHERE id = $1`

    result, err := s.db.ExecContext(ctx, query, id)
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }
    if rows == 0 {
        return errors.New("fight not found")
    }

    return nil
}

func (s *FightService) UpdateFightResults(ctx context.Context, fightID string, result *models.FightResult) error {
    query := `
        UPDATE fights
        SET fight_details = jsonb_build_object(
            'winner_id', $1,
            'finish_type', $2,
            'rounds_completed', $3,
            'round_time', $4,
            'finish_details', $5
        )
        WHERE id = $6`

    res, err := s.db.ExecContext(
        ctx,
        query,
        result.WinnerID,
        result.Method,
        result.Round,
        result.Time,
        result.Description,
        fightID,
    )
    if err != nil {
        return err
    }

    rows, err := res.RowsAffected()
    if err != nil {
        return err
    }
    if rows == 0 {
        return errors.New("fight not found")
    }

    return nil
}

func (s *FightService) GetEventFights(ctx context.Context, eventID string) ([]*models.Fight, error) {
    query := `
        SELECT id, event_id, fighter_1, fighter_2, weight_class,
               is_main_event, fight_order
        FROM fights
        WHERE event_id = $1
        ORDER BY fight_order ASC
    `

    rows, err := s.db.QueryContext(ctx, query, eventID)
    if err != nil {
        return nil, fmt.Errorf("query event fights: %w", err)
    }
    defer rows.Close()

    var fights []*models.Fight
    for rows.Next() {
        var f models.Fight
        err := rows.Scan(
            &f.ID,
            &f.EventID,
            &f.Fighter1,
            &f.Fighter2,
            &f.WeightClass,
            &f.IsMainEvent,
            &f.Order,
        )
        if err != nil {
            return nil, fmt.Errorf("scan fight: %w", err)
        }
        fights = append(fights, &f)
    }

    return fights, nil
}

func (s *FightService) DeleteOrphanedFights(ctx context.Context) error {
    query := `
        DELETE FROM fights
        WHERE event_id NOT IN (SELECT id FROM events)
    `

    _, err := s.db.ExecContext(ctx, query)
    if err != nil {
        return fmt.Errorf("delete orphaned fights: %w", err)
    }

    return nil
}