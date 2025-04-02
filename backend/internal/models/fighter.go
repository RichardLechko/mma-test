package models

import "time"

type Fighter struct {
    ID             string    `json:"id"`
    UFCID          string    `json:"ufc_id"`            
    Name           string    `json:"name"`
    Nickname       string    `json:"nickname"`
    Record         Record    `json:"record"`
    WeightClass    string    `json:"weight_class"`
    Rank           string    `json:"rank"`              
    Status         string    `json:"status"`           
    FirstRound     int       `json:"first_round"`       
    FightingOutOf  string    `json:"fighting_out_of"`  
    Height         string    `json:"height"`          
    Weight         string    `json:"weight"`          
    Age            int       `json:"age"`               
    CreatedAt      time.Time `json:"created_at"`      
    UpdatedAt      time.Time `json:"updated_at"`      
}

type Record struct {
    Wins          int `json:"wins"`
    Losses        int `json:"losses"`
    Draws         int `json:"draws"`
    KOWins        int `json:"ko_wins"`
    SubWins       int `json:"sub_wins"`
    DecWins       int `json:"dec_wins"`        
    NoContests    int `json:"no_contests"`      
    LossByKO      int `json:"loss_by_ko"`       
    LossBySub     int `json:"loss_by_sub"`      
}