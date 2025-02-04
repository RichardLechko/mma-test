package handlers

import (
	"encoding/json"
	"log"
	"mma-scheduler/cache"
	"mma-scheduler/internal/models"
	"net/http"

	"github.com/gorilla/mux"
)

type EventHandler struct {
	cache *cache.Cache
	// TODO: Add database service
}

func NewEventHandler(cache *cache.Cache) *EventHandler {
	return &EventHandler{
		cache: cache,
	}
}

func (h *EventHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	events, err := h.cache.GetUpcomingEvents(ctx)
	if err != nil {
		// TODO: Fetch from database if not in cache
		log.Printf("Error fetching events: %v", err)
		http.Error(w, "Error fetching events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// GetEvent handles GET /events/{id}
func (h *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["id"]

	ctx := r.Context()
	event, err := h.cache.GetEvent(ctx, eventID)
	if err != nil {
		// TODO: Fetch from database if not in cache
		log.Printf("Error fetching event: %v", err)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// CreateEvent handles POST /events
func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Save to database

	ctx := r.Context()
	if err := h.cache.SetEvent(ctx, &event); err != nil {
		log.Printf("Error caching event: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(event)
}

func (h *EventHandler) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["id"]

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	event.ID = eventID

	// TODO: Update in database

	ctx := r.Context()
	if err := h.cache.SetEvent(ctx, &event); err != nil {
		log.Printf("Error updating event cache: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

// DeleteEvent handles DELETE /events/{id}
func (h *EventHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	eventID := vars["id"]

	// TODO: Delete from database

	ctx := r.Context()
	if err := h.cache.Delete(ctx, "event:"+eventID); err != nil {
		log.Printf("Error removing event from cache: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
