package jobs

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"mma-scheduler/internal/models"
	"mma-scheduler/internal/services"
)

type RankingsJob struct {
	logger           *log.Logger
	fighterService   services.FighterServiceInterface
	fightService     services.FightServiceInterface
	rankingService   services.RankingServiceInterface
	promotionService services.PromotionServiceInterface
}

type RankingConfig struct {
	WinPoints          float64
	LossPoints         float64
	DrawPoints         float64
	TitleWinBonus      float64
	FinishBonus        float64
	InactivityPenalty  float64
	MaxInactiveMonths  int
	ConsiderableFights int
}

func NewRankingsJob(
	logger *log.Logger,
	fighterService services.FighterServiceInterface,
	fightService services.FightServiceInterface,
	rankingService services.RankingServiceInterface,
	promotionService services.PromotionServiceInterface,
) *RankingsJob {
	return &RankingsJob{
		logger:           logger,
		fighterService:   fighterService,
		fightService:     fightService,
		rankingService:   rankingService,
		promotionService: promotionService,
	}
}

func (j *RankingsJob) UpdateRankings(ctx context.Context) error {
	j.logger.Println("Starting rankings update")

	promotions, err := j.promotionService.GetActivePromotions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get promotions: %w", err)
	}

	errChan := make(chan error, len(promotions))
	var wg sync.WaitGroup

	for _, promotion := range promotions {
		wg.Add(1)
		go func(p *models.Promotion) {
			defer wg.Done()
			if err := j.updatePromotionRankings(ctx, p); err != nil {
				errChan <- fmt.Errorf("failed to update rankings for promotion %s: %w", p.Name, err)
			}
		}(promotion)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("rankings update cancelled: %w", ctx.Err())
	case err := <-errChan:
		if err != nil {
			return err
		}
	case <-done:
		j.logger.Println("Rankings update completed successfully")
	}

	if err := j.rankingService.RefreshRankingViews(ctx); err != nil {
		return fmt.Errorf("failed to refresh ranking views: %w", err)
	}

	return nil
}

func (j *RankingsJob) updatePromotionRankings(ctx context.Context, promotion *models.Promotion) error {
	weightClasses, err := j.rankingService.GetActiveWeightClasses(ctx, promotion.ID)
	if err != nil {
		return err
	}

	for _, weightClass := range weightClasses {
		if err := j.updateWeightClassRankings(ctx, promotion, weightClass); err != nil {
			j.logger.Printf("Error updating rankings for %s %s: %v", promotion.Name, weightClass, err)
			continue
		}
	}

	return nil
}

func (j *RankingsJob) updateWeightClassRankings(ctx context.Context, promotion *models.Promotion, weightClass string) error {
	fighters, err := j.fighterService.GetFightersByWeightClass(ctx, weightClass)
	if err != nil {
		return err
	}

	rankedFighters := make([]*models.FighterRanking, 0, len(fighters))
	for _, fighter := range fighters {
		points, err := j.calculateFighterPoints(ctx, fighter)
		if err != nil {
			j.logger.Printf("Error calculating points for fighter %s: %v", fighter.ID, err)
			continue
		}

		rankedFighters = append(rankedFighters, &models.FighterRanking{
			FighterID: fighter.ID,
			Points:    points,
		})
	}

	sort.Slice(rankedFighters, func(i, j int) bool {
		return rankedFighters[i].Points > rankedFighters[j].Points
	})

	return j.updateRankingsInDatabase(ctx, promotion.ID, weightClass, rankedFighters)
}

func (j *RankingsJob) calculateFighterPoints(ctx context.Context, fighter *models.Fighter) (float64, error) {
	config := &RankingConfig{
		WinPoints:          10.0,
		LossPoints:         -5.0,
		DrawPoints:         2.0,
		TitleWinBonus:      5.0,
		FinishBonus:        2.0,
		InactivityPenalty:  -1.0,
		MaxInactiveMonths:  12,
		ConsiderableFights: 5,
	}

	fights, err := j.fightService.GetFighterRecentFights(ctx, fighter.ID, config.ConsiderableFights)
	if err != nil {
		return 0, err
	}

	var totalPoints float64

	for _, fight := range fights {
		points := j.calculateFightPoints(fight, fighter.ID, config)
		totalPoints += points
	}

	if inactivePenalty := j.calculateInactivityPenalty(fighter, config); inactivePenalty < 0 {
		totalPoints += inactivePenalty
	}

	return totalPoints, nil
}

func (j *RankingsJob) calculateFightPoints(fight *models.Fight, fighterID string, config *RankingConfig) float64 {
	var points float64

	if fight.Result == nil {
		return points
	}

	if fight.Result.WinnerID == fighterID {
		points = config.WinPoints
		if fight.IsTitleFight {
			points += config.TitleWinBonus
		}
		if fight.Result.Method != "decision" {
			points += config.FinishBonus
		}
	} else if fight.Result.Method == "draw" {
		points = config.DrawPoints
	} else {
		points = config.LossPoints
	}

	monthsAgo := time.Since(fight.Date).Hours() / 24 / 30
	recencyMultiplier := 1.0 - (monthsAgo * 0.02)
	if recencyMultiplier < 0.5 {
		recencyMultiplier = 0.5
	}

	return points * recencyMultiplier
}

func (j *RankingsJob) calculateInactivityPenalty(fighter *models.Fighter, config *RankingConfig) float64 {
	if fighter.LastFightDate == nil {
		return config.InactivityPenalty * float64(config.MaxInactiveMonths)
	}

	monthsInactive := time.Since(*fighter.LastFightDate).Hours() / 24 / 30
	if monthsInactive > float64(config.MaxInactiveMonths) {
		return config.InactivityPenalty * monthsInactive
	}

	return 0
}

func (j *RankingsJob) updateRankingsInDatabase(ctx context.Context, promotionID, weightClass string, rankings []*models.FighterRanking) error {
	tx, err := j.rankingService.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	previousRankings, err := j.rankingService.GetCurrentRankings(ctx, promotionID, weightClass)
	if err != nil {
		return err
	}

	for rank, fighter := range rankings {
		rankUpdate := &models.RankingUpdate{
			FighterID:    fighter.FighterID,
			PromotionID:  promotionID,
			WeightClass:  weightClass,
			CurrentRank:  rank + 1,
			Points:       fighter.Points,
			PreviousRank: j.findPreviousRank(previousRankings, fighter.FighterID),
		}

		if err := j.rankingService.UpdateRanking(ctx, tx, rankUpdate); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit ranking updates: %w", err)
	}

	return nil
}

func (j *RankingsJob) findPreviousRank(previousRankings []*models.Ranking, fighterID string) int {
	for _, r := range previousRankings {
		if r.FighterID == fighterID {
			return r.Rank
		}
	}
	return 0
}
