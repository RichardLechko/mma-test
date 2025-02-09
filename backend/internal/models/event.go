package models

import (
	"time"
)

type Event struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Date        time.Time `json:"date"`
    Location    string    `json:"location"`
    Promotion   string    `json:"promotion"`
    MainCard    []Fight   `json:"main_card"`
    PrelimCard  []Fight   `json:"prelim_card"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
