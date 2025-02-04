package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"mma-scheduler/internal/services"
)

type MetricsJob struct {
    logger          *log.Logger
    fighterService  *services.FighterService
    fightService    *services.FightService
    eventService    *services.EventService
    db              *services.Database
}

type MetricType string

const (
    MetricFighterStats MetricType = "fighter_stats"
    MetricEventStats   MetricType = "event_stats"
    MetricFinishStats  MetricType = "finish_stats"
)

func NewMetricsJob(
    logger *log.Logger,
    fighterService *services.FighterService,
    fightService *services.FightService,
    eventService *services.EventService,
    db *services.Database,
) *MetricsJob {
    return &MetricsJob{
        logger:          logger,
        fighterService:  fighterService,
        fightService:    fightService,
        eventService:    eventService,
        db:              db,
    }
}

func (j *MetricsJob) UpdateMetrics(ctx context.Context) error {
    j.logger.Println("Starting metrics update")

    errChan := make(chan error, 3)
    var wg sync.WaitGroup

    wg.Add(3)
    go j.updateFighterMetrics(ctx, &wg, errChan)
    go j.updateEventMetrics(ctx, &wg, errChan)
    go j.updateFinishMetrics(ctx, &wg, errChan)

    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-ctx.Done():
        return fmt.Errorf("metrics update cancelled: %w", ctx.Err())
    case err := <-errChan:
        if err != nil {
            return fmt.Errorf("metrics update error: %w", err)
        }
    case <-done:
        j.logger.Println("Metrics update completed successfully")
    }

    if err := j.refreshMetricViews(ctx); err != nil {
        return fmt.Errorf("failed to refresh metric views: %w", err)
    }

    return nil
}

func (j *MetricsJob) updateFighterMetrics(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
    defer wg.Done()

    metrics := struct {
        TotalActiveFighters     int               `json:"total_active_fighters"`
        AverageAge             float64           `json:"average_age"`
        WeightClassDistribution map[string]int    `json:"weight_class_distribution"`
        WinMethodDistribution   map[string]int    `json:"win_method_distribution"`
    }{
        WeightClassDistribution: make(map[string]int),
        WinMethodDistribution:   make(map[string]int),
    }

    fighters, err := j.fighterService.GetActiveFighters(ctx)
    if err != nil {
        errChan <- fmt.Errorf("failed to get active fighters: %w", err)
        return
    }

    metrics.TotalActiveFighters = len(fighters)
    var totalAge float64
    var fightersWithAge int

    for _, fighter := range fighters {
        if fighter.WeightClass != "" {
            metrics.WeightClassDistribution[fighter.WeightClass]++
        }

        age := time.Since(fighter.DateOfBirth).Hours() / 24 / 365
        if age > 0 {
            totalAge += age
            fightersWithAge++
        }

        metrics.WinMethodDistribution["wins"] += fighter.Record.Wins
        metrics.WinMethodDistribution["losses"] += fighter.Record.Losses
        metrics.WinMethodDistribution["draws"] += fighter.Record.Draws
        metrics.WinMethodDistribution["no_contests"] += fighter.Record.NoContests
    }

    if fightersWithAge > 0 {
        metrics.AverageAge = totalAge / float64(fightersWithAge)
    }

    if err := j.storeMetrics(ctx, MetricFighterStats, metrics); err != nil {
        errChan <- fmt.Errorf("failed to store fighter metrics: %w", err)
        return
    }
}

func (j *MetricsJob) updateEventMetrics(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
    defer wg.Done()

    metrics := struct {
        TotalEvents          int               `json:"total_events"`
        AverageAttendance    float64           `json:"average_attendance"`
        AveragePPVBuys       float64           `json:"average_ppv_buys"`
        VenueDistribution    map[string]int    `json:"venue_distribution"`
        TitleFightCount     int               `json:"title_fight_count"`
    }{
        VenueDistribution: make(map[string]int),
    }

    startDate := time.Now().AddDate(-1, 0, 0)
    events, err := j.eventService.GetEventsSince(ctx, startDate)
    if err != nil {
        errChan <- fmt.Errorf("failed to get events: %w", err)
        return
    }

    metrics.TotalEvents = len(events)
    var totalAttendance, totalPPVBuys float64
    var eventsWithAttendance, eventsWithPPV int

    for _, event := range events {
        if event.Venue != "" {
            metrics.VenueDistribution[event.Venue]++
        }

        if event.Attendance > 0 {
            totalAttendance += float64(event.Attendance)
            eventsWithAttendance++
        }

        if event.PPVBuys > 0 {
            totalPPVBuys += float64(event.PPVBuys)
            eventsWithPPV++
        }

        titleFights, err := j.fightService.GetTitleFights(ctx, event.ID)
        if err != nil {
            j.logger.Printf("Error getting title fights for event %s: %v", event.ID, err)
            continue
        }
        metrics.TitleFightCount += len(titleFights)
    }

    if eventsWithAttendance > 0 {
        metrics.AverageAttendance = totalAttendance / float64(eventsWithAttendance)
    }
    if eventsWithPPV > 0 {
        metrics.AveragePPVBuys = totalPPVBuys / float64(eventsWithPPV)
    }

    if err := j.storeMetrics(ctx, MetricEventStats, metrics); err != nil {
        errChan <- fmt.Errorf("failed to store event metrics: %w", err)
    }
}

func (j *MetricsJob) updateFinishMetrics(ctx context.Context, wg *sync.WaitGroup, errChan chan error) {
    defer wg.Done()

    metrics := struct {
        TotalFights    int               `json:"total_fights"`
        FinishMethods  map[string]int    `json:"finish_methods"`
        RoundCount     map[int]int       `json:"round_count"`
    }{
        FinishMethods: make(map[string]int),
        RoundCount:    make(map[int]int),
    }

    filters := map[string]interface{}{
        "status": "completed",
    }
    
    fights, err := j.fightService.ListFights(ctx, filters)
    if err != nil {
        errChan <- fmt.Errorf("failed to get fights: %w", err)
        return
    }

    metrics.TotalFights = len(fights)

    for _, fight := range fights {
        if fight.Result != nil {
            metrics.FinishMethods[fight.Result.Method]++
            if fight.Result.Round > 0 {
                metrics.RoundCount[fight.Result.Round]++
            }
        }
    }

    if err := j.storeMetrics(ctx, MetricFinishStats, metrics); err != nil {
        errChan <- fmt.Errorf("failed to store finish metrics: %w", err)
    }
}

func (j *MetricsJob) storeMetrics(ctx context.Context, metricType MetricType, metrics interface{}) error {
    tx, err := j.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelDefault,
        ReadOnly:  false,
    })
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    query := `
        INSERT INTO metrics (type, data, created_at)
        VALUES ($1, $2, NOW())
    `
    
    if _, err := tx.ExecContext(ctx, query, string(metricType), metrics); err != nil {
        return fmt.Errorf("failed to store metrics: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }

    return nil
}

func (j *MetricsJob) refreshMetricViews(ctx context.Context) error {
    query := `REFRESH MATERIALIZED VIEW CONCURRENTLY fighter_ranking_summary`
    if _, err := j.db.ExecContext(ctx, query); err != nil {
        return fmt.Errorf("failed to refresh fighter ranking view: %w", err)
    }

    return nil
}