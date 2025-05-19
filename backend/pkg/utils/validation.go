package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func ValidateRequired(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return &ValidationError{
			Field:   field,
			Message: "this field is required",
		}
	}
	return nil
}

func ValidateEnum(field, value string, allowedValues []string) error {
	if !Contains(allowedValues, value) {
		return &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be one of: %s", strings.Join(allowedValues, ", ")),
		}
	}
	return nil
}

func ValidateDate(field string, date time.Time) error {
	if date.IsZero() {
		return &ValidationError{
			Field:   field,
			Message: "invalid date format",
		}
	}
	return nil
}

func ValidatePositiveNumber(field string, value float64) error {
	if value <= 0 {
		return &ValidationError{
			Field:   field,
			Message: "must be a positive number",
		}
	}
	return nil
}

var weightClasses = []string{
	"strawweight", "flyweight", "bantamweight", "featherweight",
	"lightweight", "welterweight", "middleweight", "light_heavyweight",
	"heavyweight", "womens_strawweight", "womens_flyweight",
	"womens_bantamweight", "womens_featherweight",
}

func ValidateWeightClass(weightClass string) error {
	return ValidateEnum("weight_class", weightClass, weightClasses)
}

func ValidateGender(gender string) error {
	return ValidateEnum("gender", gender, []string{"male", "female"})
}

func ValidateStance(stance string) error {
	return ValidateEnum("stance", stance, []string{"orthodox", "southpaw", "switch"})
}

func ValidateFightResult(result string) error {
	return ValidateEnum("result", result, []string{"win", "loss", "draw", "no_contest"})
}

func ValidateFinishType(finishType string) error {
	validTypes := []string{
		"knockout", "technical_knockout", "submission",
		"decision", "technical_decision", "disqualification",
	}
	return ValidateEnum("finish_type", finishType, validTypes)
}

func ValidateWeight(weight float64) error {
	if weight < 105 || weight > 300 {
		return &ValidationError{
			Field:   "weight",
			Message: "weight must be between 105 and 300 pounds",
		}
	}
	return nil
}

func ValidateHeight(height float64) error {
	if height < 120 || height > 240 {
		return &ValidationError{
			Field:   "height",
			Message: "height must be between 120 and 240 cm",
		}
	}
	return nil
}

func ValidateRounds(rounds int) error {
	if rounds != 3 && rounds != 5 {
		return &ValidationError{
			Field:   "rounds",
			Message: "number of rounds must be either 3 or 5",
		}
	}
	return nil
}

func ValidateRank(rank int) error {
	if rank < 1 || rank > 15 {
		return &ValidationError{
			Field:   "rank",
			Message: "rank must be between 1 and 15",
		}
	}
	return nil
}

func ValidateURL(field, url string) error {
	if url == "" {
		return nil
	}

	urlRegex := regexp.MustCompile(`^https?:\/\/.+`)
	if !urlRegex.MatchString(url) {
		return &ValidationError{
			Field:   field,
			Message: "invalid URL format",
		}
	}
	return nil
}
