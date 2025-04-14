package services

import (
    "context"
    "time"
    "database/sql"

    "mma-scheduler/internal/models"
)

type DatabaseService interface {
    // Database operations
    GetDB() *sql.DB
    Close() error
    Ping(ctx context.Context) error
    
    // Event operations
    GetEvents(ctx context.Context, limit int) ([]*models.Event, error)
    GetEventByID(ctx context.Context, id string) (*models.Event, error)
    CreateEvent(ctx context.Context, event *models.Event) error
    UpdateEvent(ctx context.Context, event *models.Event) error
    DeleteEvent(ctx context.Context, id string) error
    
    // Fight operations
    GetFights(ctx context.Context, eventID string) ([]*models.Fight, error)
    GetFightByID(ctx context.Context, id string) (*models.Fight, error)
    CreateFight(ctx context.Context, fight *models.Fight) error
    UpdateFight(ctx context.Context, fight *models.Fight) error
    DeleteFight(ctx context.Context, id string) error
}

type EventServiceInterface interface {
	GetUpcomingEvents(ctx context.Context) ([]*models.Event, error)
	GetEventsSince(ctx context.Context, since time.Time) ([]*models.Event, error)
	GetEventByID(ctx context.Context, id string) (*models.Event, error)
	GetEventByUFCURL(ctx context.Context, ufcURL string) (*models.Event, error)
	CreateEvent(ctx context.Context, event *models.Event) error
	UpdateEvent(ctx context.Context, event *models.Event) error
	DeleteEvent(ctx context.Context, id string) error
}

type ScraperServiceInterface interface {
    ScrapeEvent(ctx context.Context, url string) (*models.Event, error)
    ScrapeUpcomingEvents(ctx context.Context) ([]models.Event, error)
}

type FightServiceInterface interface {
    CreateFight(ctx context.Context, fight *models.Fight) error
    GetFightByID(ctx context.Context, id string) (*models.Fight, error)
    UpdateFight(ctx context.Context, fight *models.Fight) error
    DeleteFight(ctx context.Context, id string) error
    GetEventFights(ctx context.Context, eventID string) ([]*models.Fight, error)
    UpdateFightResults(ctx context.Context, fightID string, result *models.FightResult) error
}