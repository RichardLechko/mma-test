package services

import (
	"context"
	"database/sql"
	"time"

	"mma-scheduler/internal/models"
)

type FighterServiceInterface interface {
	GetActiveFighters(ctx context.Context) ([]*models.Fighter, error)
	GetFightersByWeightClass(ctx context.Context, weightClass string) ([]*models.Fighter, error)
	GetFightersToUpdate(ctx context.Context) ([]*models.Fighter, error)
	UpdateFighter(ctx context.Context, fighter *models.Fighter) error

	GetDuplicateFighters(ctx context.Context) ([][]*models.Fighter, error)
	MergeFighters(ctx context.Context, mainID string, duplicates []*models.Fighter) error
	ValidateRecords(ctx context.Context) error
}

type FightServiceInterface interface {
	GetFighterFights(ctx context.Context, fighterID string) ([]*models.Fight, error)
	GetTitleFights(ctx context.Context, eventID string) ([]*models.Fight, error)
	GetEventFights(ctx context.Context, eventID string) ([]*models.Fight, error)
	GetFighterRecentFights(ctx context.Context, fighterID string, limit int) ([]*models.Fight, error)
	GetFightsWithoutResults(ctx context.Context) ([]*models.Fight, error)
	GetFightsSince(ctx context.Context, since time.Time) ([]*models.Fight, error)
	UpdateFightResults(ctx context.Context, fightID string, results *models.FightResult) error

	ArchiveOldFights(ctx context.Context, cutoffDate time.Time) error
	ValidateFightResults(ctx context.Context) error
}

type EventServiceInterface interface {
	GetUpcomingEvents(ctx context.Context) ([]*models.Event, error)
	GetEventsSince(ctx context.Context, since time.Time) ([]*models.Event, error)
	GetPromotionEventsSince(ctx context.Context, promotionID string, since time.Time) ([]*models.Event, error)
	UpdateEvent(ctx context.Context, event *models.Event) error

	GetDuplicateEvents(ctx context.Context) ([][]*models.Event, error)
	DeleteEvent(ctx context.Context, eventID string) error
	ValidateEventData(ctx context.Context) error
}

type PromotionServiceInterface interface {
	GetActivePromotions(ctx context.Context) ([]*models.Promotion, error)
}

type RankingServiceInterface interface {
	GetActiveWeightClasses(ctx context.Context, promotionID string) ([]string, error)
	GetCurrentRankings(ctx context.Context, promotionID string, weightClass string) ([]*models.Ranking, error)
	UpdateRanking(ctx context.Context, tx *sql.Tx, update *models.RankingUpdate) error
	RefreshRankingViews(ctx context.Context) error
	BeginTx(ctx context.Context) (*sql.Tx, error)
	DeleteOrphanedRankings(ctx context.Context) error
}

type ScraperServiceInterface interface {
	ScrapeFighter(ctx context.Context, fighterID string) (*models.Fighter, error)
	ScrapeEvent(ctx context.Context, eventID string) (*models.Event, error)
	ScrapeFightResults(ctx context.Context, fightID string) (*models.FightResult, error)
}

type MetricsService interface {
	StoreMetrics(ctx context.Context, tx *sql.Tx, metricType string, metrics interface{}) error
	RefreshMaterializedView(ctx context.Context, viewName string) error
}

type DatabaseService interface {
	BeginTx(ctx context.Context) (*sql.Tx, error)
	StoreMetrics(ctx context.Context, tx *sql.Tx, metricType string, metrics interface{}) error
	RefreshMaterializedView(ctx context.Context, viewName string) error
}
