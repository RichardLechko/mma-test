package models

import (
	"time"
)

type Fighter struct {
    ID              string     `json:"id"`
    FullName        string     `json:"full_name"`
    Nickname        string    `json:"nickname,omitempty"`
    Gender          string    `json:"gender"`
    WeightClass     string    `json:"weight_class"`
    DateOfBirth     time.Time `json:"date_of_birth,omitempty"`
    Height          float64   `json:"height,omitempty"`
    Weight          float64   `json:"weight,omitempty"`
    Reach          float64   `json:"reach,omitempty"`
    Stance         string    `json:"stance,omitempty"`
    Active         bool      `json:"active"`
    Record         Record    `json:"record"`
	LastFightDate   *time.Time `json:"last_fight_date,omitempty"`
}

type Record struct {
    Wins        int `json:"wins"`
    Losses      int `json:"losses"`
    Draws       int `json:"draws"`
    NoContests  int `json:"no_contests"`
}