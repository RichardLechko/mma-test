package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var (
	notificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_sent_total",
			Help: "Total number of notifications sent by type",
		},
		[]string{"type"},
	)

	notificationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_errors_total",
			Help: "Total number of notification errors by type",
		},
		[]string{"type", "error"},
	)
)

type NotificationType string

const (
	NotificationUpcomingFight  NotificationType = "upcoming_fight"
	NotificationFightResult    NotificationType = "fight_result"
	NotificationEventScheduled NotificationType = "event_scheduled"
	NotificationRankingChange  NotificationType = "ranking_change"
)

type Notification struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Message   string           `json:"message"`
	Data      interface{}      `json:"data,omitempty"`
	Read      bool             `json:"read"`
	CreatedAt time.Time        `json:"created_at"`
}

type NotificationService struct {
	redis *redis.Client
}

func NewNotificationService(redis *redis.Client) *NotificationService {
	return &NotificationService{
		redis: redis,
	}
}

func (s *NotificationService) SendNotification(ctx context.Context, notification *Notification) error {
	notification.CreatedAt = time.Now()
	notification.Read = false

	notifJSON, err := json.Marshal(notification)
	if err != nil {
		notificationErrors.WithLabelValues(string(notification.Type), "marshal").Inc()
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	key := fmt.Sprintf("notifications:%s", notification.UserID)
	err = s.redis.LPush(ctx, key, notifJSON).Err()
	if err != nil {
		notificationErrors.WithLabelValues(string(notification.Type), "store").Inc()
		return fmt.Errorf("failed to store notification: %w", err)
	}

	s.redis.LTrim(ctx, key, 0, 99)

	notificationsSent.WithLabelValues(string(notification.Type)).Inc()
	return nil
}

func (s *NotificationService) GetUserNotifications(ctx context.Context, userID string, limit int) ([]Notification, error) {
	key := fmt.Sprintf("notifications:%s", userID)

	results, err := s.redis.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}

	var notifications []Notification
	for _, result := range results {
		var notification Notification
		if err := json.Unmarshal([]byte(result), &notification); err != nil {
			continue
		}
		notifications = append(notifications, notification)
	}

	return notifications, nil
}

func (s *NotificationService) MarkAsRead(ctx context.Context, userID, notificationID string) error {
	key := fmt.Sprintf("notifications:%s", userID)

	results, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("failed to get notifications: %w", err)
	}

	for i, result := range results {
		var notification Notification
		if err := json.Unmarshal([]byte(result), &notification); err != nil {
			continue
		}

		if notification.ID == notificationID {
			notification.Read = true
			notifJSON, err := json.Marshal(notification)
			if err != nil {
				return fmt.Errorf("failed to marshal notification: %w", err)
			}

			err = s.redis.LSet(ctx, key, int64(i), notifJSON).Err()
			if err != nil {
				return fmt.Errorf("failed to update notification: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("notification not found")
}

func (s *NotificationService) CreateUpcomingFightNotification(ctx context.Context, userID, fighter1, fighter2, eventName string, fightTime time.Time) error {
	notification := &Notification{
		ID:     fmt.Sprintf("notif_%d", time.Now().UnixNano()),
		UserID: userID,
		Type:   NotificationUpcomingFight,
		Title:  "Upcoming Fight",
		Message: fmt.Sprintf("%s vs %s at %s",
			fighter1,
			fighter2,
			eventName,
		),
		Data: map[string]interface{}{
			"fighter1":  fighter1,
			"fighter2":  fighter2,
			"eventName": eventName,
			"fightTime": fightTime,
		},
	}

	return s.SendNotification(ctx, notification)
}

func (s *NotificationService) CreateFightResultNotification(ctx context.Context, userID, winner, loser, method string) error {
	notification := &Notification{
		ID:     fmt.Sprintf("notif_%d", time.Now().UnixNano()),
		UserID: userID,
		Type:   NotificationFightResult,
		Title:  "Fight Result",
		Message: fmt.Sprintf("%s defeated %s by %s",
			winner,
			loser,
			method,
		),
		Data: map[string]interface{}{
			"winner": winner,
			"loser":  loser,
			"method": method,
		},
	}

	return s.SendNotification(ctx, notification)
}

func (s *NotificationService) ClearOldNotifications(ctx context.Context, userID string, olderThan time.Duration) error {
	key := fmt.Sprintf("notifications:%s", userID)

	results, err := s.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("failed to get notifications: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	for i, result := range results {
		var notification Notification
		if err := json.Unmarshal([]byte(result), &notification); err != nil {
			continue
		}

		if notification.CreatedAt.Before(cutoff) {

			err = s.redis.LTrim(ctx, key, 0, int64(i-1)).Err()
			if err != nil {
				return fmt.Errorf("failed to trim notifications: %w", err)
			}
			break
		}
	}

	return nil
}
