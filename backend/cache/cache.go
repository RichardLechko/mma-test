package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"mma-scheduler/internal/models"

	"github.com/redis/go-redis/v9"
)

type CacheConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type Cache struct {
	client *redis.Client
}

const (
	DefaultExpiration  = 24 * time.Hour
	FighterExpiration  = 12 * time.Hour
	EventExpiration    = 1 * time.Hour
	RankingExpiration  = 6 * time.Hour
	UpcomingExpiration = 30 * time.Minute
)

const (
	KeyFighter   = "fighter:%s"
	KeyEvent     = "event:%s"
	KeyPromotion = "promotion:%s"
	KeyUpcoming  = "upcoming:events"
	KeyRankings  = "rankings:%s:%s"
)

const (
	KeyFight       = "fight:%s"
	KeyEventFights = "event:%s:fights"
)

const (
	KeyRanking             = "ranking:%s"
	KeyWeightClassRankings = "rankings:%s:%s"
	KeyFighterRankings     = "fighter:%s:rankings"
)

func NewCache(config CacheConfig) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &Cache{client: client}, nil
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %w", err)
	}

	return c.client.Set(ctx, key, data, expiration).Err()
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("cache miss for key %s", key)
		}
		return fmt.Errorf("failed to get cache value: %w", err)
	}

	return json.Unmarshal(data, dest)
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *Cache) GetFighter(ctx context.Context, fighterID string) (*models.Fighter, error) {
	var fighter models.Fighter
	key := fmt.Sprintf(KeyFighter, fighterID)

	err := c.Get(ctx, key, &fighter)
	if err != nil {
		return nil, err
	}

	return &fighter, nil
}

func (c *Cache) SetFighter(ctx context.Context, fighter *models.Fighter) error {
	key := fmt.Sprintf(KeyFighter, fighter.ID)
	return c.Set(ctx, key, fighter, FighterExpiration)
}

func (c *Cache) GetUpcomingEvents(ctx context.Context) ([]models.Event, error) {
	var events []models.Event
	err := c.Get(ctx, KeyUpcoming, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *Cache) SetUpcomingEvents(ctx context.Context, events []models.Event) error {
	return c.Set(ctx, KeyUpcoming, events, UpcomingExpiration)
}

type CacheService struct {
	cache *Cache
}

func (s *CacheService) GetFighterWithCache(ctx context.Context, fighterID string) (*models.Fighter, error) {
	fighter, err := s.cache.GetFighter(ctx, fighterID)
	if err == nil {
		return fighter, nil
	}

	fighter, err = s.getFighterFromDB(fighterID)
	if err != nil {
		return nil, err
	}

	if err := s.cache.SetFighter(ctx, fighter); err != nil {
		fmt.Printf("failed to cache fighter: %v\n", err)
	}

	return fighter, nil
}

func (s *CacheService) getFighterFromDB(fighterID string) (*models.Fighter, error) {
	// TODO: Implement database retrieval
	return nil, fmt.Errorf("not implemented")
}

func (c *Cache) GetEvent(ctx context.Context, eventID string) (*models.Event, error) {
	var event models.Event
	key := fmt.Sprintf(KeyEvent, eventID)

	err := c.Get(ctx, key, &event)
	if err != nil {
		return nil, err
	}

	return &event, nil
}

func (c *Cache) SetEvent(ctx context.Context, event *models.Event) error {
	key := fmt.Sprintf(KeyEvent, event.ID)
	return c.Set(ctx, key, event, EventExpiration)
}

func (c *Cache) GetFight(ctx context.Context, fightID string) (*models.Fight, error) {
	var fight models.Fight
	key := fmt.Sprintf(KeyFight, fightID)

	err := c.Get(ctx, key, &fight)
	if err != nil {
		return nil, err
	}

	return &fight, nil
}

func (c *Cache) SetFight(ctx context.Context, fight *models.Fight) error {
	key := fmt.Sprintf(KeyFight, fight.ID)
	return c.Set(ctx, key, fight, DefaultExpiration)
}

func (c *Cache) GetEventFights(ctx context.Context, eventID string) ([]models.Fight, error) {
	var fights []models.Fight
	key := fmt.Sprintf(KeyEventFights, eventID)

	err := c.Get(ctx, key, &fights)
	if err != nil {
		return nil, err
	}

	return fights, nil
}

func (c *Cache) SetEventFights(ctx context.Context, eventID string, fights []models.Fight) error {
	key := fmt.Sprintf(KeyEventFights, eventID)
	return c.Set(ctx, key, fights, DefaultExpiration)
}

func (c *Cache) GetRanking(ctx context.Context, rankingID string) (*models.Ranking, error) {
	var ranking models.Ranking
	key := fmt.Sprintf(KeyRanking, rankingID)

	err := c.Get(ctx, key, &ranking)
	if err != nil {
		return nil, err
	}

	return &ranking, nil
}

func (c *Cache) SetRanking(ctx context.Context, ranking *models.Ranking) error {
	key := fmt.Sprintf(KeyRanking, ranking.ID)
	return c.Set(ctx, key, ranking, RankingExpiration)
}

func (c *Cache) GetWeightClassRankings(ctx context.Context, promotionID, weightClass string) ([]models.Ranking, error) {
	var rankings []models.Ranking
	key := fmt.Sprintf(KeyWeightClassRankings, promotionID, weightClass)

	err := c.Get(ctx, key, &rankings)
	if err != nil {
		return nil, err
	}

	return rankings, nil
}

func (c *Cache) SetWeightClassRankings(ctx context.Context, promotionID, weightClass string, rankings []models.Ranking) error {
	key := fmt.Sprintf(KeyWeightClassRankings, promotionID, weightClass)
	return c.Set(ctx, key, rankings, RankingExpiration)
}

func (c *Cache) GetFighterRankings(ctx context.Context, fighterID string) ([]models.Ranking, error) {
	var rankings []models.Ranking
	key := fmt.Sprintf(KeyFighterRankings, fighterID)

	err := c.Get(ctx, key, &rankings)
	if err != nil {
		return nil, err
	}

	return rankings, nil
}

func (c *Cache) SetFighterRankings(ctx context.Context, fighterID string, rankings []models.Ranking) error {
	key := fmt.Sprintf(KeyFighterRankings, fighterID)
	return c.Set(ctx, key, rankings, RankingExpiration)
}
