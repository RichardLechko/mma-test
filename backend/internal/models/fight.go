package models

import (
	"time"
)

type Fight struct {
    ID              string       `json:"id"`
    EventID         string       `json:"event_id"`
    Fighter1ID      string       `json:"fighter1_id"`
    Fighter2ID      string       `json:"fighter2_id"`
    WeightClass     string       `json:"weight_class"`
    IsTitleFight    bool         `json:"is_title_fight"`
    IsMainEvent     bool         `json:"is_main_event"`
    Status          string       `json:"status"`
    ScheduledRounds int          `json:"scheduled_rounds"`
    Date            time.Time    `json:"date"`
    Result          *FightResult `json:"result,omitempty"`
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