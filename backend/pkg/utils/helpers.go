package utils

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func ParseDate(date string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"January 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, date); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", date)
}

func CreateSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "'", "")
	return strings.ReplaceAll(s, ".", "")
}

func StructToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func FormatWeight(weight float64) string {
	return fmt.Sprintf("%.1f", weight)
}

func FormatRecord(wins, losses, draws, noContests int) string {
	record := fmt.Sprintf("%d-%d-%d", wins, losses, draws)
	if noContests > 0 {
		record += fmt.Sprintf(" (%d NC)", noContests)
	}
	return record
}

func ParseDuration(duration string) (int, error) {
	parts := strings.Split(duration, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid duration format: %s", duration)
	}

	var minutes, seconds int
	_, err := fmt.Sscanf(duration, "%d:%d", &minutes, &seconds)
	if err != nil {
		return 0, err
	}

	return minutes*60 + seconds, nil
}

func FormatDuration(seconds int) string {
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	return fmt.Sprintf("%d:%02d", minutes, remainingSeconds)
}

func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
