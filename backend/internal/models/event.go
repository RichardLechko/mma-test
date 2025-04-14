package models

import (
	"time"
)

type Event struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Date        time.Time `json:"date"`
    Venue       string    `json:"venue"`
    City        string    `json:"city"`
    Country     string    `json:"country"`
    Status      string    `json:"status"`
    WikiURL     string    `json:"wiki_url"`
    UFCURL      string    `json:"ufc_url"`
    Promotion   string    `json:"promotion"`
    MainCard    []Fight   `json:"main_card"`
    PrelimCard  []Fight   `json:"prelim_card"`
    Attendance  string    `json:"attendance"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}