package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"mma-scheduler/internal/models"
	"mma-scheduler/internal/services"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var (
	userOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_operations_total",
			Help: "Total number of user operations by type",
		},
		[]string{"operation"},
	)

	userOperationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_operation_errors_total",
			Help: "Total number of user operation errors by type",
		},
		[]string{"operation", "error"},
	)
)

type UserHandler struct {
	db       *sql.DB
	redis    *redis.Client
	notifSvc *services.NotificationService
}

func NewUserHandler(db *sql.DB, redis *redis.Client, notifSvc *services.NotificationService) *UserHandler {
	return &UserHandler{
		db:       db,
		redis:    redis,
		notifSvc: notifSvc,
	}
}

func (h *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	userOperations.WithLabelValues("register").Inc()

	var input struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		userOperationErrors.WithLabelValues("register", "decode").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user := models.NewUser(input.Email, input.Username)
	user.FirstName = input.FirstName
	user.LastName = input.LastName

	const query = `
		INSERT INTO users (email, username, first_name, last_name, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	err := h.db.QueryRow(
		query,
		user.Email,
		user.Username,
		user.FirstName,
		user.LastName,
		input.Password,
		time.Now(),
		time.Now(),
	).Scan(&user.ID)

	if err != nil {
		userOperationErrors.WithLabelValues("register", "db").Inc()
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userOperations.WithLabelValues("update_profile").Inc()

	userID := mux.Vars(r)["id"]
	var input struct {
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		userOperationErrors.WithLabelValues("update_profile", "decode").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	const query = `
		UPDATE users 
		SET username = $1, first_name = $2, last_name = $3, updated_at = $4
		WHERE id = $5
		RETURNING *`

	var user models.User
	err := h.db.QueryRow(
		query,
		input.Username,
		input.FirstName,
		input.LastName,
		time.Now(),
		userID,
	).Scan(&user.ID, &user.Email, &user.Username, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		userOperationErrors.WithLabelValues("update_profile", "db").Inc()
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userOperations.WithLabelValues("update_preferences").Inc()

	userID := mux.Vars(r)["id"]
	var prefs models.UserPreferences

	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		userOperationErrors.WithLabelValues("update_preferences", "decode").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	const query = `
		UPDATE users 
		SET user_preferences = $1, updated_at = $2
		WHERE id = $3
		RETURNING id`

	err := h.db.QueryRow(query, prefs, time.Now(), userID).Scan(&userID)
	if err != nil {
		userOperationErrors.WithLabelValues("update_preferences", "db").Inc()
		http.Error(w, "Failed to update preferences", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prefs)
}

func (h *UserHandler) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	userOperations.WithLabelValues("update_notifications").Inc()

	userID := mux.Vars(r)["id"]
	var notifPrefs models.NotificationPreferences

	if err := json.NewDecoder(r.Body).Decode(&notifPrefs); err != nil {
		userOperationErrors.WithLabelValues("update_notifications", "decode").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	const query = `
		UPDATE users 
		SET notification_preferences = $1, updated_at = $2
		WHERE id = $3
		RETURNING id`

	err := h.db.QueryRow(query, notifPrefs, time.Now(), userID).Scan(&userID)
	if err != nil {
		userOperationErrors.WithLabelValues("update_notifications", "db").Inc()
		http.Error(w, "Failed to update notification settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifPrefs)
}

func (h *UserHandler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	userOperations.WithLabelValues("get_profile").Inc()

	userID := mux.Vars(r)["id"]

	const query = `
		SELECT id, email, username, first_name, last_name, created_at, updated_at, 
			   user_preferences, notification_preferences
		FROM users 
		WHERE id = $1`

	var user models.User
	err := h.db.QueryRow(query, userID).Scan(
		&user.ID, &user.Email, &user.Username, &user.FirstName, &user.LastName,
		&user.CreatedAt, &user.UpdatedAt, &user.UserPreferences, &user.NotificationPrefs,
	)

	if err != nil {
		userOperationErrors.WithLabelValues("get_profile", "db").Inc()
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userOperations.WithLabelValues("delete").Inc()

	userID := mux.Vars(r)["id"]

	const query = `DELETE FROM users WHERE id = $1`

	result, err := h.db.Exec(query, userID)
	if err != nil {
		userOperationErrors.WithLabelValues("delete", "db").Inc()
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
