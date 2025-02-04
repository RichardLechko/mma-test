package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mma-scheduler/internal/models"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var (
	searchRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "search_requests_total",
			Help: "Total number of search requests by type",
		},
		[]string{"type"},
	)

	searchLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "search_latency_seconds",
			Help:    "Search operation latency in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0},
		},
		[]string{"type"},
	)
)

type SearchOptions struct {
	Query       string
	WeightClass string
	Status      string
	DateFrom    time.Time
	DateTo      time.Time
	Limit       int
	Offset      int
}

type SearchResults struct {
	Fighters []models.Fighter `json:"fighters,omitempty"`
	Events   []models.Event   `json:"events,omitempty"`
	Fights   []models.Fight   `json:"fights,omitempty"`
	Total    int              `json:"total"`
}

type SearchService struct {
	db    *sql.DB
	redis *redis.Client
}

func NewSearchService(db *sql.DB, redis *redis.Client) *SearchService {
	return &SearchService{
		db:    db,
		redis: redis,
	}
}

func (s *SearchService) Search(ctx context.Context, opts SearchOptions) (*SearchResults, error) {
	start := time.Now()
	defer func() {
		searchLatency.WithLabelValues("global").Observe(time.Since(start).Seconds())
	}()

	searchRequests.WithLabelValues("global").Inc()

	cacheKey := s.buildCacheKey(opts)
	cachedResult, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var results SearchResults
		if err := json.Unmarshal([]byte(cachedResult), &results); err == nil {
			return &results, nil
		}
	}

	results := &SearchResults{}

	results.Fighters, err = s.SearchFighters(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("fighter search error: %w", err)
	}

	results.Events, err = s.SearchEvents(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("event search error: %w", err)
	}

	results.Fights, err = s.SearchFights(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("fight search error: %w", err)
	}

	results.Total = len(results.Fighters) + len(results.Events) + len(results.Fights)

	if resultJSON, err := json.Marshal(results); err == nil {
		s.redis.Set(ctx, cacheKey, resultJSON, 5*time.Minute)
	}

	return results, nil
}

func (s *SearchService) SearchFighters(ctx context.Context, opts SearchOptions) ([]models.Fighter, error) {
	start := time.Now()
	defer func() {
		searchLatency.WithLabelValues("fighters").Observe(time.Since(start).Seconds())
	}()

	searchRequests.WithLabelValues("fighters").Inc()

	query := `
		SELECT * FROM fighters
		WHERE search_vector @@ plainto_tsquery($1)
	`
	params := []interface{}{opts.Query}

	if opts.WeightClass != "" {
		query += " AND weight_class = $2"
		params = append(params, opts.WeightClass)
	}

	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("fighter search query error: %w", err)
	}
	defer rows.Close()

	var fighters []models.Fighter
	for rows.Next() {
		var f models.Fighter
		if err := rows.Scan(&f.ID, &f.FullName, &f.WeightClass, &f.Record); err != nil {
			return nil, fmt.Errorf("error scanning fighter row: %w", err)
		}
		fighters = append(fighters, f)
	}

	return fighters, nil
}

func (s *SearchService) SearchEvents(ctx context.Context, opts SearchOptions) ([]models.Event, error) {
	start := time.Now()
	defer func() {
		searchLatency.WithLabelValues("events").Observe(time.Since(start).Seconds())
	}()

	searchRequests.WithLabelValues("events").Inc()

	query := `
		SELECT * FROM events 
		WHERE name ILIKE $1 
	`
	params := []interface{}{fmt.Sprintf("%%%s%%", opts.Query)}

	if !opts.DateFrom.IsZero() {
		query += " AND event_date >= $2"
		params = append(params, opts.DateFrom)
	}

	if !opts.DateTo.IsZero() {
		query += " AND event_date <= $3"
		params = append(params, opts.DateTo)
	}

	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("event search query error: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.Name, &e.Date, &e.Status); err != nil {
			return nil, fmt.Errorf("error scanning event row: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

func (s *SearchService) SearchFights(ctx context.Context, opts SearchOptions) ([]models.Fight, error) {
	start := time.Now()
	defer func() {
		searchLatency.WithLabelValues("fights").Observe(time.Since(start).Seconds())
	}()

	searchRequests.WithLabelValues("fights").Inc()

	query := `
		SELECT f.* FROM fights f
		JOIN fighters f1 ON f.fighter1_id = f1.id
		JOIN fighters f2 ON f.fighter2_id = f2.id
		WHERE f1.full_name ILIKE $1 OR f2.full_name ILIKE $1
	`
	params := []interface{}{fmt.Sprintf("%%%s%%", opts.Query)}

	if opts.WeightClass != "" {
		query += " AND f.weight_class = $2"
		params = append(params, opts.WeightClass)
	}

	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("fight search query error: %w", err)
	}
	defer rows.Close()

	var fights []models.Fight
	for rows.Next() {
		var f models.Fight
		if err := rows.Scan(&f.ID, &f.EventID, &f.Fighter1ID, &f.Fighter2ID, &f.WeightClass, &f.Status); err != nil {
			return nil, fmt.Errorf("error scanning fight row: %w", err)
		}
		fights = append(fights, f)
	}

	return fights, nil
}

func (s *SearchService) buildCacheKey(opts SearchOptions) string {
	parts := []string{
		"search",
		opts.Query,
		opts.WeightClass,
		opts.Status,
	}
	if !opts.DateFrom.IsZero() {
		parts = append(parts, opts.DateFrom.Format("2006-01-02"))
	}
	if !opts.DateTo.IsZero() {
		parts = append(parts, opts.DateTo.Format("2006-01-02"))
	}
	return strings.Join(parts, ":")
}
