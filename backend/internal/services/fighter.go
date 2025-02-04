package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mma-scheduler/internal/models"

	"github.com/google/uuid"
)

type FighterService struct {
	db *sql.DB
}

func (s *FighterService) GetActiveFighters(ctx context.Context) ([]*models.Fighter, error) {
	query := `
        SELECT 
            id, 
            full_name, 
            weight_class,
            date_of_birth,
            nickname,
            gender,
            height,
            weight,
            reach,
            stance,
            active
        FROM fighters 
        WHERE active = true
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fighters []*models.Fighter
	for rows.Next() {
		fighter := &models.Fighter{}
		err := rows.Scan(
			&fighter.ID,
			&fighter.FullName,
			&fighter.WeightClass,
			&fighter.DateOfBirth,
			&fighter.Nickname,
			&fighter.Gender,
			&fighter.Height,
			&fighter.Weight,
			&fighter.Reach,
			&fighter.Stance,
			&fighter.Active,
		)
		if err != nil {
			return nil, err
		}
		fighters = append(fighters, fighter)
	}

	return fighters, nil
}

func NewFighterService(db *sql.DB) FighterServiceInterface {
	return &FighterService{
		db: db,
	}
}

func (s *FighterService) CreateFighter(ctx context.Context, fighter *models.Fighter) error {
	query := `
		INSERT INTO fighters (
			id, full_name, nickname, gender, weight_class, 
			date_of_birth, height, weight, reach, stance, active,
			record
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
			jsonb_build_object(
				'wins', $12,
				'losses', $13,
				'draws', $14,
				'no_contests', $15
			)
		) RETURNING id`

	id := uuid.New().String()
	err := s.db.QueryRowContext(
		ctx,
		query,
		id,
		fighter.FullName,
		fighter.Nickname,
		fighter.Gender,
		fighter.WeightClass,
		fighter.DateOfBirth,
		fighter.Height,
		fighter.Weight,
		fighter.Reach,
		fighter.Stance,
		fighter.Active,
		fighter.Record.Wins,
		fighter.Record.Losses,
		fighter.Record.Draws,
		fighter.Record.NoContests,
	).Scan(&fighter.ID)

	if err != nil {
		return err
	}

	return nil
}

func (s *FighterService) GetFighterByID(ctx context.Context, id string) (*models.Fighter, error) {
	query := `
		SELECT 
			id, full_name, nickname, gender, weight_class,
			date_of_birth, height, weight, reach, stance, active,
			record->>'wins' as wins,
			record->>'losses' as losses,
			record->>'draws' as draws,
			record->>'no_contests' as no_contests
		FROM fighters
		WHERE id = $1`

	fighter := &models.Fighter{}
	var record models.Record

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&fighter.ID,
		&fighter.FullName,
		&fighter.Nickname,
		&fighter.Gender,
		&fighter.WeightClass,
		&fighter.DateOfBirth,
		&fighter.Height,
		&fighter.Weight,
		&fighter.Reach,
		&fighter.Stance,
		&fighter.Active,
		&record.Wins,
		&record.Losses,
		&record.Draws,
		&record.NoContests,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("fighter not found")
	}
	if err != nil {
		return nil, err
	}

	fighter.Record = record
	return fighter, nil
}

func (s *FighterService) ListFighters(ctx context.Context, filters map[string]interface{}) ([]models.Fighter, error) {
	query := `
		SELECT 
			id, full_name, nickname, gender, weight_class,
			date_of_birth, height, weight, reach, stance, active,
			record->>'wins' as wins,
			record->>'losses' as losses,
			record->>'draws' as draws,
			record->>'no_contests' as no_contests
		FROM fighters
		WHERE ($1::text IS NULL OR weight_class = $1)
		AND ($2::text IS NULL OR gender = $2)
		AND ($3::boolean IS NULL OR active = $3)
		ORDER BY full_name`

	rows, err := s.db.QueryContext(ctx, query,
		filters["weight_class"],
		filters["gender"],
		filters["active"],
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fighters []models.Fighter
	for rows.Next() {
		var fighter models.Fighter
		var record models.Record
		err := rows.Scan(
			&fighter.ID,
			&fighter.FullName,
			&fighter.Nickname,
			&fighter.Gender,
			&fighter.WeightClass,
			&fighter.DateOfBirth,
			&fighter.Height,
			&fighter.Weight,
			&fighter.Reach,
			&fighter.Stance,
			&fighter.Active,
			&record.Wins,
			&record.Losses,
			&record.Draws,
			&record.NoContests,
		)
		if err != nil {
			return nil, err
		}
		fighter.Record = record
		fighters = append(fighters, fighter)
	}

	return fighters, nil
}

func (s *FighterService) UpdateFighter(ctx context.Context, fighter *models.Fighter) error {
	query := `
		UPDATE fighters
		SET 
			full_name = $1,
			nickname = $2,
			gender = $3,
			weight_class = $4,
			date_of_birth = $5,
			height = $6,
			weight = $7,
			reach = $8,
			stance = $9,
			active = $10,
			record = jsonb_build_object(
				'wins', $11,
				'losses', $12,
				'draws', $13,
				'no_contests', $14
			)
		WHERE id = $15`

	result, err := s.db.ExecContext(
		ctx,
		query,
		fighter.FullName,
		fighter.Nickname,
		fighter.Gender,
		fighter.WeightClass,
		fighter.DateOfBirth,
		fighter.Height,
		fighter.Weight,
		fighter.Reach,
		fighter.Stance,
		fighter.Active,
		fighter.Record.Wins,
		fighter.Record.Losses,
		fighter.Record.Draws,
		fighter.Record.NoContests,
		fighter.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("fighter not found")
	}

	return nil
}

func (s *FighterService) DeleteFighter(ctx context.Context, id string) error {
	query := `DELETE FROM fighters WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("fighter not found")
	}

	return nil
}

func (s *FighterService) SearchFighters(ctx context.Context, searchTerm string) ([]models.Fighter, error) {
	query := `
		SELECT 
			id, full_name, nickname, gender, weight_class,
			date_of_birth, height, weight, reach, stance, active,
			record->>'wins' as wins,
			record->>'losses' as losses,
			record->>'draws' as draws,
			record->>'no_contests' as no_contests
		FROM fighters
		WHERE 
			search_vector @@ plainto_tsquery($1)
			OR full_name ILIKE $2
			OR nickname ILIKE $2
		ORDER BY full_name`

	searchPattern := "%" + searchTerm + "%"
	rows, err := s.db.QueryContext(ctx, query, searchTerm, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fighters []models.Fighter
	for rows.Next() {
		var fighter models.Fighter
		var record models.Record
		err := rows.Scan(
			&fighter.ID,
			&fighter.FullName,
			&fighter.Nickname,
			&fighter.Gender,
			&fighter.WeightClass,
			&fighter.DateOfBirth,
			&fighter.Height,
			&fighter.Weight,
			&fighter.Reach,
			&fighter.Stance,
			&fighter.Active,
			&record.Wins,
			&record.Losses,
			&record.Draws,
			&record.NoContests,
		)
		if err != nil {
			return nil, err
		}
		fighter.Record = record
		fighters = append(fighters, fighter)
	}

	return fighters, nil
}

func (s *FighterService) UpdateFighterRecord(ctx context.Context, fighterID string, record *models.Record) error {
	query := `
		UPDATE fighters
		SET record = jsonb_build_object(
			'wins', $1,
			'losses', $2,
			'draws', $3,
			'no_contests', $4
		)
		WHERE id = $5`

	result, err := s.db.ExecContext(
		ctx,
		query,
		record.Wins,
		record.Losses,
		record.Draws,
		record.NoContests,
		fighterID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("fighter not found")
	}

	return nil
}

func (s *FighterService) GetFightersByWeightClass(ctx context.Context, weightClass string) ([]*models.Fighter, error) {
	query := `
        SELECT id, full_name, weight_class, date_of_birth
        FROM fighters 
        WHERE weight_class = $1 AND active = true
    `
	rows, err := s.db.QueryContext(ctx, query, weightClass)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fighters []*models.Fighter
	for rows.Next() {
		f := &models.Fighter{}
		if err := rows.Scan(&f.ID, &f.FullName, &f.WeightClass, &f.DateOfBirth); err != nil {
			return nil, err
		}
		fighters = append(fighters, f)
	}
	return fighters, nil
}

func (s *FighterService) GetDuplicateFighters(ctx context.Context) ([][]*models.Fighter, error) {
	query := `
        WITH duplicate_fighters AS (
            SELECT 
                array_agg(id) as ids,
                full_name,
                COUNT(*) as count
            FROM fighters
            GROUP BY full_name
            HAVING COUNT(*) > 1
        )
        SELECT id, full_name, nickname, gender, weight_class, date_of_birth, height, weight, reach, stance, active
        FROM fighters
        WHERE id IN (
            SELECT unnest(ids)
            FROM duplicate_fighters
        )
        ORDER BY full_name, id
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query duplicate fighters: %w", err)
	}
	defer rows.Close()

	fighterMap := make(map[string][]*models.Fighter)

	for rows.Next() {
		var f models.Fighter
		err := rows.Scan(
			&f.ID,
			&f.FullName,
			&f.Nickname,
			&f.Gender,
			&f.WeightClass,
			&f.DateOfBirth,
			&f.Height,
			&f.Weight,
			&f.Reach,
			&f.Stance,
			&f.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("scan fighter: %w", err)
		}

		fighterMap[f.FullName] = append(fighterMap[f.FullName], &f)
	}

	var result [][]*models.Fighter
	for _, fighters := range fighterMap {
		result = append(result, fighters)
	}

	return result, nil
}

func (s *FighterService) MergeFighters(ctx context.Context, mainID string, duplicates []*models.Fighter) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, dup := range duplicates {

		if _, err := tx.ExecContext(ctx,
			"UPDATE fights SET fighter1_id = $1 WHERE fighter1_id = $2",
			mainID, dup.ID); err != nil {
			return fmt.Errorf("update fighter1_id: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE fights SET fighter2_id = $1 WHERE fighter2_id = $2",
			mainID, dup.ID); err != nil {
			return fmt.Errorf("update fighter2_id: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE rankings SET fighter_id = $1 WHERE fighter_id = $2",
			mainID, dup.ID); err != nil {
			return fmt.Errorf("update rankings: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			"DELETE FROM fighters WHERE id = $1",
			dup.ID); err != nil {
			return fmt.Errorf("delete duplicate fighter: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *FighterService) ValidateRecords(ctx context.Context) error {
	query := `
        WITH fight_counts AS (
            SELECT 
                fighter_id,
                COUNT(*) FILTER (WHERE result = 'win') as wins,
                COUNT(*) FILTER (WHERE result = 'loss') as losses,
                COUNT(*) FILTER (WHERE result = 'draw') as draws,
                COUNT(*) FILTER (WHERE result = 'no_contest') as no_contests
            FROM fight_results
            GROUP BY fighter_id
        )
        UPDATE fighters f
        SET record = jsonb_build_object(
            'wins', fc.wins,
            'losses', fc.losses,
            'draws', fc.draws,
            'no_contests', fc.no_contests
        )
        FROM fight_counts fc
        WHERE f.id = fc.fighter_id
        AND (
            f.record->>'wins' != fc.wins::text
            OR f.record->>'losses' != fc.losses::text
            OR f.record->>'draws' != fc.draws::text
            OR f.record->>'no_contests' != fc.no_contests::text
        )
    `

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("validate fighter records: %w", err)
	}

	return nil
}

func (s *FighterService) GetFightersToUpdate(ctx context.Context) ([]*models.Fighter, error) {
	query := `
        SELECT id, full_name, nickname, gender, weight_class, date_of_birth, 
               height, weight, reach, stance, active
        FROM fighters
        WHERE active = true
        AND (updated_at < NOW() - INTERVAL '24 hours' 
             OR updated_at IS NULL)
        LIMIT 100
    `

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query fighters to update: %w", err)
	}
	defer rows.Close()

	var fighters []*models.Fighter
	for rows.Next() {
		var f models.Fighter
		err := rows.Scan(
			&f.ID,
			&f.FullName,
			&f.Nickname,
			&f.Gender,
			&f.WeightClass,
			&f.DateOfBirth,
			&f.Height,
			&f.Weight,
			&f.Reach,
			&f.Stance,
			&f.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("scan fighter: %w", err)
		}
		fighters = append(fighters, &f)
	}

	return fighters, nil
}

func (s *FightService) GetFighterFights(ctx context.Context, fighterID string) ([]*models.Fight, error) {
	query := `
        SELECT id, event_id, fighter1_id, fighter2_id, weight_class,
               is_title_fight, is_main_event, status, scheduled_rounds, date
        FROM fights
        WHERE fighter1_id = $1 OR fighter2_id = $1
        ORDER BY date DESC
    `

	rows, err := s.db.QueryContext(ctx, query, fighterID)
	if err != nil {
		return nil, fmt.Errorf("query fighter fights: %w", err)
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
