package services

import (
	"context"
	"database/sql"
	"fmt"

	"mma-scheduler/internal/models"
)

type RankingService struct {
	db *sql.DB
}

func NewRankingService(db *sql.DB) RankingServiceInterface {
	return &RankingService{db: db}
}

func (s *RankingService) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}

func (s *RankingService) RefreshRankingViews(ctx context.Context) error {
	query := "REFRESH MATERIALIZED VIEW fighter_ranking_summary"
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *RankingService) GetActiveWeightClasses(ctx context.Context, promotionID string) ([]string, error) {
	query := `
        SELECT DISTINCT weight_class
        FROM rankings
        WHERE promotion_id = $1
        AND status = 'active'
        ORDER BY weight_class
    `

	rows, err := s.db.QueryContext(ctx, query, promotionID)
	if err != nil {
		return nil, fmt.Errorf("query weight classes: %w", err)
	}
	defer rows.Close()

	var classes []string
	for rows.Next() {
		var class string
		if err := rows.Scan(&class); err != nil {
			return nil, err
		}
		classes = append(classes, class)
	}

	return classes, nil
}

func (s *RankingService) GetCurrentRankings(ctx context.Context, promotionID string, weightClass string) ([]*models.Ranking, error) {
	query := `
        SELECT id, fighter_id, rank, points, previous_rank, effective_date
        FROM rankings
        WHERE promotion_id = $1
        AND weight_class = $2
        AND status = 'active'
        ORDER BY rank
    `

	rows, err := s.db.QueryContext(ctx, query, promotionID, weightClass)
	if err != nil {
		return nil, fmt.Errorf("query rankings: %w", err)
	}
	defer rows.Close()

	var rankings []*models.Ranking
	for rows.Next() {
		var r models.Ranking
		err := rows.Scan(
			&r.ID,
			&r.FighterID,
			&r.Rank,
			&r.Points,
			&r.PreviousRank,
			&r.EffectiveDate,
		)
		if err != nil {
			return nil, err
		}
		rankings = append(rankings, &r)
	}

	return rankings, nil
}

func (s *RankingService) UpdateRanking(ctx context.Context, tx *sql.Tx, update *models.RankingUpdate) error {
	query := `
        INSERT INTO rankings (
            fighter_id,
            promotion_id,
            weight_class,
            rank,
            previous_rank,
            points,
            effective_date,
            status
        ) VALUES (
            $1, $2, $3, $4, $5, $6, NOW(), 'active'
        )
        ON CONFLICT (fighter_id, promotion_id, weight_class) 
        DO UPDATE SET
            rank = EXCLUDED.rank,
            previous_rank = rankings.rank,
            points = EXCLUDED.points,
            effective_date = NOW(),
            updated_at = NOW()
        WHERE rankings.status = 'active'
    `

	_, err := tx.ExecContext(ctx, query,
		update.FighterID,
		update.PromotionID,
		update.WeightClass,
		update.CurrentRank,
		update.PreviousRank,
		update.Points,
	)

	if err != nil {
		return fmt.Errorf("update ranking: %w", err)
	}

	return nil
}

func (s *RankingService) DeleteOrphanedRankings(ctx context.Context) error {
	query := `
        UPDATE rankings 
        SET status = 'inactive',
            updated_at = NOW()
        WHERE fighter_id IN (
            SELECT r.fighter_id
            FROM rankings r
            LEFT JOIN fighters f ON f.id = r.fighter_id
            WHERE f.id IS NULL
               OR f.active = false
               OR f.current_weight_class != r.weight_class
        )
        AND status = 'active'
    `

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("delete orphaned rankings: %w", err)
	}

	return nil
}
