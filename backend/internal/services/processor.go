package services

import (
	"context"
	"fmt"

	"mma-scheduler/internal/models"
	"mma-scheduler/pkg/utils"
)

type ProcessorService struct {
	fighterService *FighterService
	eventService   *EventService
}

func NewProcessorService(fighterService *FighterService, eventService *EventService) *ProcessorService {
	return &ProcessorService{
		fighterService: fighterService,
		eventService:   eventService,
	}
}

func (s *ProcessorService) ProcessFighterData(ctx context.Context, fighter *models.Fighter) error {
	if err := utils.ValidateRequired("full_name", fighter.FullName); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if err := utils.ValidateWeightClass(fighter.WeightClass); err != nil {
		return fmt.Errorf("invalid weight class: %w", err)
	}

	if fighter.Height > 0 {
		if err := utils.ValidateHeight(fighter.Height); err != nil {
			return fmt.Errorf("invalid height: %w", err)
		}
	}

	if fighter.Weight > 0 {
		if err := utils.ValidateWeight(fighter.Weight); err != nil {
			return fmt.Errorf("invalid weight: %w", err)
		}
	}

	if fighter.Stance != "" {
		if err := utils.ValidateStance(fighter.Stance); err != nil {
			return fmt.Errorf("invalid stance: %w", err)
		}
	}

	return nil
}

func (s *ProcessorService) ProcessEventData(ctx context.Context, event *models.Event) error {
	if err := utils.ValidateRequired("name", event.Name); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if event.Date.IsZero() {
		return fmt.Errorf("event date is required")
	}

	for i := range event.Fights {
		if err := s.ProcessFightData(ctx, &event.Fights[i]); err != nil {
			return fmt.Errorf("error processing fight %d: %w", i+1, err)
		}
	}

	return nil
}

func (s *ProcessorService) ProcessFightData(ctx context.Context, fight *models.Fight) error {
	if err := utils.ValidateWeightClass(fight.WeightClass); err != nil {
		return fmt.Errorf("invalid weight class: %w", err)
	}

	if fight.Fighter1ID == "" || fight.Fighter2ID == "" {
		return fmt.Errorf("both fighters must be specified")
	}

	if fight.Fighter1ID == fight.Fighter2ID {
		return fmt.Errorf("fighter cannot fight themselves")
	}

	if fight.ScheduledRounds != 3 && fight.ScheduledRounds != 5 {
		return fmt.Errorf("scheduled rounds must be either 3 or 5")
	}

	if fight.Result != nil {
		if err := s.validateFightResult(fight.Result); err != nil {
			return fmt.Errorf("invalid fight result: %w", err)
		}
	}

	return nil
}

func (s *ProcessorService) ProcessRankingData(ctx context.Context, ranking *models.Ranking) error {
	if err := utils.ValidateWeightClass(ranking.WeightClass); err != nil {
		return fmt.Errorf("invalid weight class: %w", err)
	}

	if err := utils.ValidateRank(ranking.Rank); err != nil {
		return fmt.Errorf("invalid rank: %w", err)
	}

	if ranking.FighterID == "" {
		return fmt.Errorf("fighter ID is required")
	}

	if ranking.PromotionID == "" {
		return fmt.Errorf("promotion ID is required")
	}

	return nil
}

func (s *ProcessorService) validateFightResult(result *models.FightResult) error {
	if result.WinnerID == "" {
		return fmt.Errorf("winner ID is required")
	}

	if result.Round < 1 || result.Round > 5 {
		return fmt.Errorf("invalid round number")
	}

	validMethods := []string{"KO", "TKO", "SUB", "DEC", "DQ", "NC"}
	methodValid := false
	for _, method := range validMethods {
		if result.Method == method {
			methodValid = true
			break
		}
	}
	if !methodValid {
		return fmt.Errorf("invalid fight method")
	}

	return nil
}
