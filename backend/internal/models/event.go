package models

import (
	"time"
)

type Event struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Date       time.Time `json:"date"`
	Venue      string    `json:"venue,omitempty"`
	City       string    `json:"city,omitempty"`
	Country    string    `json:"country,omitempty"`
	Status     string    `json:"status"`
	Fights     []Fight   `json:"fights,omitempty"`
	Attendance int       `json:"attendance"`
	PPVBuys    int       `json:"ppv_buys"`
}
