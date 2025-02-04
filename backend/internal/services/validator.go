package services

import (
	"context"
	"fmt"

	"mma-scheduler/internal/models"
	"mma-scheduler/pkg/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	validationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "data_validation_errors_total",
			Help: "Total number of validation errors by entity type and error type",
		},
		[]string{"entity_type", "error_type"},
	)

	validationChecks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "data_validation_checks_total",
			Help: "Total number of validation checks performed by entity type",
		},
		[]string{"entity_type"},
	)
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidatorService struct {
	versionService *VersionService
}

func NewValidatorService(versionService *VersionService) *ValidatorService {
	return &ValidatorService{
		versionService: versionService,
	}
}

func (s *ValidatorService) ValidateFighter(ctx context.Context, fighter *models.Fighter) []ValidationError {
	var errors []ValidationError
	validationChecks.WithLabelValues("fighter").Inc()

	if err := utils.ValidateRequired("full_name", fighter.FullName); err != nil {
		errors = append(errors, ValidationError{
			Field:   "full_name",
			Message: err.Error(),
			Type:    "required",
		})
		validationErrors.WithLabelValues("fighter", "required").Inc()
	}

	if err := utils.ValidateWeightClass(fighter.WeightClass); err != nil {
		errors = append(errors, ValidationError{
			Field:   "weight_class",
			Message: err.Error(),
			Type:    "invalid",
		})
		validationErrors.WithLabelValues("fighter", "weight_class").Inc()
	}

	if fighter.Height > 0 {
		if err := utils.ValidateHeight(fighter.Height); err != nil {
			errors = append(errors, ValidationError{
				Field:   "height",
				Message: err.Error(),
				Type:    "range",
			})
			validationErrors.WithLabelValues("fighter", "height").Inc()
		}
	}

	if fighter.Weight > 0 {
		if err := utils.ValidateWeight(fighter.Weight); err != nil {
			errors = append(errors, ValidationError{
				Field:   "weight",
				Message: err.Error(),
				Type:    "range",
			})
			validationErrors.WithLabelValues("fighter", "weight").Inc()
		}
	}

	return errors
}

func (s *ValidatorService) ValidateEvent(ctx context.Context, event *models.Event) []ValidationError {
	var errors []ValidationError
	validationChecks.WithLabelValues("event").Inc()

	if err := utils.ValidateRequired("name", event.Name); err != nil {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: err.Error(),
			Type:    "required",
		})
		validationErrors.WithLabelValues("event", "required").Inc()
	}

	if event.Date.IsZero() {
		errors = append(errors, ValidationError{
			Field:   "date",
			Message: "event date is required",
			Type:    "required",
		})
		validationErrors.WithLabelValues("event", "date").Inc()
	}

	for i, fight := range event.Fights {
		if fightErrors := s.ValidateFight(ctx, &fight); len(fightErrors) > 0 {
			for _, err := range fightErrors {
				err.Field = fmt.Sprintf("fights[%d].%s", i, err.Field)
				errors = append(errors, err)
			}
		}
	}

	return errors
}

func (s *ValidatorService) ValidateFight(ctx context.Context, fight *models.Fight) []ValidationError {
	var errors []ValidationError
	validationChecks.WithLabelValues("fight").Inc()

	if err := utils.ValidateWeightClass(fight.WeightClass); err != nil {
		errors = append(errors, ValidationError{
			Field:   "weight_class",
			Message: err.Error(),
			Type:    "invalid",
		})
		validationErrors.WithLabelValues("fight", "weight_class").Inc()
	}

	if fight.Fighter1ID == fight.Fighter2ID {
		errors = append(errors, ValidationError{
			Field:   "fighters",
			Message: "fighter cannot fight themselves",
			Type:    "logic",
		})
		validationErrors.WithLabelValues("fight", "logic").Inc()
	}

	if fight.ScheduledRounds != 3 && fight.ScheduledRounds != 5 {
		errors = append(errors, ValidationError{
			Field:   "scheduled_rounds",
			Message: "scheduled rounds must be either 3 or 5",
			Type:    "range",
		})
		validationErrors.WithLabelValues("fight", "rounds").Inc()
	}

	return errors
}

func (s *ValidatorService) ValidateRanking(ctx context.Context, ranking *models.Ranking) []ValidationError {
	var errors []ValidationError
	validationChecks.WithLabelValues("ranking").Inc()

	if err := utils.ValidateWeightClass(ranking.WeightClass); err != nil {
		errors = append(errors, ValidationError{
			Field:   "weight_class",
			Message: err.Error(),
			Type:    "invalid",
		})
		validationErrors.WithLabelValues("ranking", "weight_class").Inc()
	}

	if err := utils.ValidateRank(ranking.Rank); err != nil {
		errors = append(errors, ValidationError{
			Field:   "rank",
			Message: err.Error(),
			Type:    "range",
		})
		validationErrors.WithLabelValues("ranking", "rank").Inc()
	}

	return errors
}
