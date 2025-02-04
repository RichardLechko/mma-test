package models

import (
	"time"
)

type User struct {
	ID                string                  `json:"id"`
	Email             string                  `json:"email"`
	PasswordHash      string                  `json:"-"`
	Username          string                  `json:"username"`
	FirstName         string                  `json:"first_name,omitempty"`
	LastName          string                  `json:"last_name,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	LastLogin         time.Time               `json:"last_login,omitempty"`
	EmailVerified     bool                    `json:"email_verified"`
	Active            bool                    `json:"active"`
	NotificationPrefs NotificationPreferences `json:"notification_preferences"`
	UserPreferences   UserPreferences         `json:"user_preferences"`
}

type NotificationPreferences struct {
	UpcomingFights bool     `json:"upcoming_fights"`
	FightResults   bool     `json:"fight_results"`
	RankingChanges bool     `json:"ranking_changes"`
	EventAlerts    bool     `json:"event_alerts"`
	EmailNotifs    bool     `json:"email_notifications"`
	PushNotifs     bool     `json:"push_notifications"`
	Fighters       []string `json:"favorite_fighters"`
}

type UserPreferences struct {
	Theme         string   `json:"theme"`
	TimeZone      string   `json:"timezone"`
	Language      string   `json:"language"`
	WeightClasses []string `json:"weight_classes"`
	Promotions    []string `json:"promotions"`
}

type UserSession struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	UserAgent    string    `json:"user_agent"`
	IPAddress    string    `json:"ip_address"`
}

type UserStats struct {
	UserID            string    `json:"user_id"`
	LastActive        time.Time `json:"last_active"`
	NotificationCount int       `json:"notification_count"`
	FavoriteFights    int       `json:"favorite_fights"`
	SavedEvents       int       `json:"saved_events"`
	LastNotification  time.Time `json:"last_notification"`
}

func NewUser(email, username string) *User {
	now := time.Now()
	return &User{
		Email:     email,
		Username:  username,
		CreatedAt: now,
		UpdatedAt: now,
		Active:    true,
		NotificationPrefs: NotificationPreferences{
			UpcomingFights: true,
			FightResults:   true,
			RankingChanges: true,
			EventAlerts:    true,
			EmailNotifs:    true,
			PushNotifs:     false,
		},
		UserPreferences: UserPreferences{
			Theme:    "light",
			TimeZone: "UTC",
			Language: "en",
		},
	}
}

func (u *User) UpdateLastLogin() {
	u.LastLogin = time.Now()
	u.UpdatedAt = time.Now()
}

func (u *User) UpdatePassword(passwordHash string) {
	u.PasswordHash = passwordHash
	u.UpdatedAt = time.Now()
}

func (u *User) IsActive() bool {
	return u.Active && u.EmailVerified
}

func (u *User) ToggleNotification(notifType string) {
	switch notifType {
	case "upcoming_fights":
		u.NotificationPrefs.UpcomingFights = !u.NotificationPrefs.UpcomingFights
	case "fight_results":
		u.NotificationPrefs.FightResults = !u.NotificationPrefs.FightResults
	case "ranking_changes":
		u.NotificationPrefs.RankingChanges = !u.NotificationPrefs.RankingChanges
	case "event_alerts":
		u.NotificationPrefs.EventAlerts = !u.NotificationPrefs.EventAlerts
	case "email":
		u.NotificationPrefs.EmailNotifs = !u.NotificationPrefs.EmailNotifs
	case "push":
		u.NotificationPrefs.PushNotifs = !u.NotificationPrefs.PushNotifs
	}
	u.UpdatedAt = time.Now()
}

func (u *User) AddFavoriteFighter(fighterID string) {
	for _, id := range u.NotificationPrefs.Fighters {
		if id == fighterID {
			return
		}
	}
	u.NotificationPrefs.Fighters = append(u.NotificationPrefs.Fighters, fighterID)
	u.UpdatedAt = time.Now()
}

func (u *User) RemoveFavoriteFighter(fighterID string) {
	fighters := make([]string, 0)
	for _, id := range u.NotificationPrefs.Fighters {
		if id != fighterID {
			fighters = append(fighters, id)
		}
	}
	u.NotificationPrefs.Fighters = fighters
	u.UpdatedAt = time.Now()
}
