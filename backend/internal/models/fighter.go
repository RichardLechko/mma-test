package models

import "time"

type Fighter struct {
    ID           string    `json:"id"`
    UFCID        string    `json:"ufc_id"`        // New: from UFC.com URL
    Name         string    `json:"name"`
    Nickname     string    `json:"nickname"`
    Record       Record    `json:"record"`
    WeightClass  string    `json:"weight_class"`
    Rank         string    `json:"rank"`          // New: e.g., "#3"
    Status       string    `json:"status"`        // New: "Active" or not
    FirstRound   int       `json:"first_round"`   // New: First round finishes
    CreatedAt    time.Time `json:"created_at"`    // New: for tracking
    UpdatedAt    time.Time `json:"updated_at"`    // New: for tracking
}

type Record struct {
    Wins         int `json:"wins"`
    Losses       int `json:"losses"`
    Draws        int `json:"draws"`
    KOWins       int `json:"ko_wins"`
    SubWins      int `json:"sub_wins"`
}