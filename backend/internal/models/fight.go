package models

import (
	"time"
)

type Fight struct {
	ID          string `json:"id"`
	EventID     string `json:"event_id"`
	Fighter1    string `json:"fighter_1"`
	Fighter2    string `json:"fighter_2"`
	WeightClass string `json:"weight_class"`
	IsMainEvent bool   `json:"is_main_event"`
	Order       int    `json:"order"`
}

type FightResult struct {
	WinnerID    string `json:"winner_id"`
	Method      string `json:"method"`
	Round       int    `json:"round"`
	Time        string `json:"time"`
	Description string `json:"description,omitempty"`
}

type WeighIn struct {
	FighterID  string    `json:"fighter_id"`
	Weight     float64   `json:"weight"`
	DateTime   time.Time `json:"date_time"`
	MadeWeight bool      `json:"made_weight"`
	Notes      string    `json:"notes,omitempty"`
}
