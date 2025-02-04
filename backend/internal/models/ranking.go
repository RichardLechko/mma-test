package models

import (
	"time"
)

type Ranking struct {
    ID           string    `json:"id"`
    FighterID string  `json:"fighter_id"`
    PromotionID  string    `json:"promotion_id"`
    WeightClass  string    `json:"weight_class"`
    Rank         int       `json:"rank"`
    PreviousRank int       `json:"previous_rank,omitempty"`
    Points    float64 `json:"points"`
    EffectiveDate time.Time `json:"effective_date"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type RankingUpdate struct {
    FighterID    string    `json:"fighter_id"`
    PromotionID  string    `json:"promotion_id"`
    WeightClass  string    `json:"weight_class"`
    CurrentRank  int       `json:"current_rank"`
    PreviousRank int       `json:"previous_rank"`
    Points       float64   `json:"points"`
}

type FighterRanking struct {
    FighterID string  `json:"fighter_id"`
    Points    float64 `json:"points"`
}