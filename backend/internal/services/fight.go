package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mma-scheduler/internal/models"
	"time"

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
			id, event_id, fighter1_id, fighter2_id, weight_class,
			is_title_fight, is_main_event, status, number_of_rounds
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id`

	id := uuid.New().String()
	err := s.db.QueryRowContext(
		ctx,
		query,
		id,
		fight.EventID,
		fight.Fighter1ID,
		fight.Fighter2ID,
		fight.WeightClass,
		fight.IsTitleFight,
		fight.IsMainEvent,
		fight.Status,
		fight.ScheduledRounds,
	).Scan(&fight.ID)

	if err != nil {
		return err
	}

	return nil
}

func (s *FightService) GetOldFights(ctx context.Context, cutoffDate time.Time, limit, offset int) ([]*models.Fight, error) {
	query := `
        SELECT id, event_id, fighter1_id, fighter2_id, weight_class,
               is_title_fight, is_main_event, status, number_of_rounds
        FROM fights
        WHERE created_at < $1
        ORDER BY created_at
        LIMIT $2 OFFSET $3
    `

	rows, err := s.db.QueryContext(ctx, query, cutoffDate, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fights []*models.Fight
	for rows.Next() {
		fight := &models.Fight{}
		err := rows.Scan(
			&fight.ID,
			&fight.EventID,
			&fight.Fighter1ID,
			&fight.Fighter2ID,
			&fight.WeightClass,
			&fight.IsTitleFight,
			&fight.IsMainEvent,
			&fight.Status,
			&fight.ScheduledRounds,
		)
		if err != nil {
			return nil, err
		}
		fights = append(fights, fight)
	}

	return fights, nil
}

func (s *FightService) GetFightByID(ctx context.Context, id string) (*models.Fight, error) {
	query := `
		SELECT 
			id, event_id, fighter1_id, fighter2_id, weight_class,
			is_title_fight, is_main_event, status, number_of_rounds,
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
		&fight.Fighter1ID,
		&fight.Fighter2ID,
		&fight.WeightClass,
		&fight.IsTitleFight,
		&fight.IsMainEvent,
		&fight.Status,
		&fight.ScheduledRounds,
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

	if result.WinnerID != "" {
		fight.Result = result
	}

	return fight, nil
}

func (s *FightService) ListFights(ctx context.Context, filters map[string]interface{}) ([]models.Fight, error) {
	query := `
		SELECT 
			id, event_id, fighter1_id, fighter2_id, weight_class,
			is_title_fight, is_main_event, status, number_of_rounds
		FROM fights
		WHERE ($1::text IS NULL OR event_id = $1)
		AND ($2::text IS NULL OR fighter1_id = $2 OR fighter2_id = $2)
		AND ($3::text IS NULL OR weight_class = $3)
		AND ($4::text IS NULL OR status = $4)
		ORDER BY fight_order DESC`

	rows, err := s.db.QueryContext(ctx, query,
		filters["event_id"],
		filters["fighter_id"],
		filters["weight_class"],
		filters["status"],
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
			&fight.Fighter1ID,
			&fight.Fighter2ID,
			&fight.WeightClass,
			&fight.IsTitleFight,
			&fight.IsMainEvent,
			&fight.Status,
			&fight.ScheduledRounds,
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
			fighter1_id = $2,
			fighter2_id = $3,
			weight_class = $4,
			is_title_fight = $5,
			is_main_event = $6,
			status = $7,
			number_of_rounds = $8
		WHERE id = $9`

	result, err := s.db.ExecContext(
		ctx,
		query,
		fight.EventID,
		fight.Fighter1ID,
		fight.Fighter2ID,
		fight.WeightClass,
		fight.IsTitleFight,
		fight.IsMainEvent,
		fight.Status,
		fight.ScheduledRounds,
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

func (s *FightService) GetFighterSchedule(ctx context.Context, fighterID string) ([]models.Fight, error) {
	query := `
		SELECT 
			f.id, f.event_id, f.fighter1_id, f.fighter2_id, 
			f.weight_class, f.is_title_fight, f.is_main_event, 
			f.status, f.number_of_rounds, e.event_date as date
		FROM fights f
		JOIN events e ON f.event_id = e.id
		WHERE (f.fighter1_id = $1 OR f.fighter2_id = $1)
		ORDER BY e.event_date DESC`

	rows, err := s.db.QueryContext(ctx, query, fighterID)
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
			&fight.Fighter1ID,
			&fight.Fighter2ID,
			&fight.WeightClass,
			&fight.IsTitleFight,
			&fight.IsMainEvent,
			&fight.Status,
			&fight.ScheduledRounds,
			&fight.Date,
		)
		if err != nil {
			return nil, err
		}
		fights = append(fights, fight)
	}

	return fights, nil
}

func (s *FightService) GetTitleFights(ctx context.Context, eventID string) ([]*models.Fight, error) {
	query := `
        SELECT id, fighter1_id, fighter2_id, weight_class
        FROM fights
        WHERE event_id = $1 AND is_title_fight = true
    `

	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fights []*models.Fight
	for rows.Next() {
		fight := &models.Fight{}
		err := rows.Scan(
			&fight.ID,
			&fight.Fighter1ID,
			&fight.Fighter2ID,
			&fight.WeightClass,
		)
		if err != nil {
			return nil, err
		}
		fights = append(fights, fight)
	}

	return fights, nil
}

func (s *FightService) GetFighterRecentFights(ctx context.Context, fighterID string, limit int) ([]*models.Fight, error) {
	query := `
        SELECT id, event_id, fighter1_id, fighter2_id, weight_class, 
               is_title_fight, is_main_event, date
        FROM fights
        WHERE (fighter1_id = $1 OR fighter2_id = $1)
        ORDER BY date DESC
        LIMIT $2
    `
	rows, err := s.db.QueryContext(ctx, query, fighterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fights []*models.Fight
	for rows.Next() {
		f := &models.Fight{}
		err := rows.Scan(&f.ID, &f.EventID, &f.Fighter1ID, &f.Fighter2ID,
			&f.WeightClass, &f.IsTitleFight, &f.IsMainEvent, &f.Date)
		if err != nil {
			return nil, err
		}
		fights = append(fights, f)
	}
	return fights, nil
}

func (s *FightService) DeleteOrphanedFights(ctx context.Context) error {
	query := `
        DELETE FROM fights
        WHERE event_id NOT IN (SELECT id FROM events)
        OR fighter1_id NOT IN (SELECT id FROM fighters)
        OR fighter2_id NOT IN (SELECT id FROM fighters)
    `

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("delete orphaned fights: %w", err)
	}

	return nil
}

func (s *FightService) ValidateFightResults(ctx context.Context) error {

	query := `
        UPDATE fights f
        SET fight_details = jsonb_set(
            fight_details,
            '{winner_id}',
            to_jsonb(fr.fighter_id)
        )
        FROM fight_results fr
        WHERE f.id = fr.fight_id
        AND fr.result = 'win'
        AND (
            f.fight_details->>'winner_id' IS NULL
            OR f.fight_details->>'winner_id' != fr.fighter_id
        )
    `

	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("validate fight winners: %w", err)
	}

	return nil
}

func (s *FightService) ArchiveOldFights(ctx context.Context, cutoffDate time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
        WITH moved_fights AS (
            DELETE FROM fights
            WHERE fight_date < $1
            RETURNING *
        )
        INSERT INTO historical_fights
        SELECT 
            id, event_id, fighter1_id, fighter2_id, fight_date, 
            weight_class, contracted_weight, is_title_fight, 
            is_interim_title, is_main_event, is_co_main_event, 
            fight_order, number_of_rounds, result, fight_details, 
            fight_stats, status, canceled_reason, created_at, updated_at
        FROM moved_fights
    `

	_, err = tx.ExecContext(ctx, query, cutoffDate)
	if err != nil {
		return fmt.Errorf("archive old fights: %w", err)
	}

	moveQueries := []string{
		`INSERT INTO historical_fight_results 
         SELECT * FROM fight_results 
         WHERE fight_id IN (
             SELECT id FROM historical_fights 
             WHERE fight_date < $1
         )`,

		`INSERT INTO historical_fight_media
         SELECT * FROM fight_media
         WHERE fight_id IN (
             SELECT id FROM historical_fights
             WHERE fight_date < $1
         )`,
	}

	for _, query := range moveQueries {
		if _, err := tx.ExecContext(ctx, query, cutoffDate); err != nil {
			return fmt.Errorf("move related data: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *FightService) GetEventFights(ctx context.Context, eventID string) ([]*models.Fight, error) {
	query := `
        SELECT id, event_id, fighter1_id, fighter2_id, weight_class,
               is_title_fight, is_main_event, status, scheduled_rounds, date
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
			&f.Fighter1ID,
			&f.Fighter2ID,
			&f.WeightClass,
			&f.IsTitleFight,
			&f.IsMainEvent,
			&f.Status,
			&f.ScheduledRounds,
			&f.Date,
		)
		if err != nil {
			return nil, fmt.Errorf("scan fight: %w", err)
		}
		fights = append(fights, &f)
	}

	return fights, nil
}

func (s *FightService) GetFightsSince(ctx context.Context, since time.Time) ([]*models.Fight, error) {
	query := `
        SELECT id, event_id, fighter1_id, fighter2_id, weight_class,
               is_title_fight, is_main_event, status, scheduled_rounds, date
        FROM fights
        WHERE date >= $1
        ORDER BY date DESC
    `

	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("query fights since %v: %w", since, err)
	}
	defer rows.Close()

	var fights []*models.Fight
	for rows.Next() {
		var f models.Fight
		err := rows.Scan(
			&f.ID,
			&f.EventID,
			&f.Fighter1ID,
			&f.Fighter2ID,
			&f.WeightClass,
			&f.IsTitleFight,
			&f.IsMainEvent,
			&f.Status,
			&f.ScheduledRounds,
			&f.Date,
		)
		if err != nil {
			return nil, fmt.Errorf("scan fight: %w", err)
		}
		fights = append(fights, &f)
	}

	return fights, nil
}

func (s *FightService) GetFightsWithoutResults(ctx context.Context) ([]*models.Fight, error) {
	query := `
        SELECT id, event_id, fighter1_id, fighter2_id, weight_class, 
               is_title_fight, is_main_event, status, number_of_rounds, 
               fight_date
        FROM fights 
        WHERE result IS NULL AND status = 'completed'
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying fights without results: %w", err)
	}
	defer rows.Close()

	var fights []*models.Fight
	for rows.Next() {
		fight := &models.Fight{}
		err := rows.Scan(
			&fight.ID,
			&fight.EventID,
			&fight.Fighter1ID,
			&fight.Fighter2ID,
			&fight.WeightClass,
			&fight.IsTitleFight,
			&fight.IsMainEvent,
			&fight.Status,
			&fight.ScheduledRounds,
			&fight.Date,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning fight row: %w", err)
		}
		fights = append(fights, fight)
	}

	return fights, nil
}
