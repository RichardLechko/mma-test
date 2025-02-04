package models

type Promotion struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ShortName   string `json:"short_name,omitempty"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active"`
}