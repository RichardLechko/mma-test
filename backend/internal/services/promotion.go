package services

import (
	"context"
	"database/sql"
	"mma-scheduler/internal/models"
)

type PromotionService struct {
    db *sql.DB
}

var _ PromotionServiceInterface = (*PromotionService)(nil)

func NewPromotionService(db *sql.DB) *PromotionService {
    return &PromotionService{
        db: db,
    }
}

func (s *PromotionService) GetActivePromotions(ctx context.Context) ([]*models.Promotion, error) {
    query := `
        SELECT id, name 
        FROM promotions 
        WHERE active = true
    `
    rows, err := s.db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var promotions []*models.Promotion
    for rows.Next() {
        p := &models.Promotion{}
        if err := rows.Scan(&p.ID, &p.Name); err != nil {
            return nil, err
        }
        promotions = append(promotions, p)
    }
    return promotions, nil
}