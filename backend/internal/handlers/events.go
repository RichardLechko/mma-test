package handlers

import (
    "encoding/json"
    "log"
    "mma-scheduler/internal/models"
    "mma-scheduler/internal/services"
    "net/http"

    "github.com/gorilla/mux"
)

type EventHandler struct {
    eventService services.EventServiceInterface
}

func NewEventHandler(eventService services.EventServiceInterface) *EventHandler {
    return &EventHandler{
        eventService: eventService,
    }
}

func (h *EventHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    events, err := h.eventService.GetUpcomingEvents(ctx)
    if err != nil {
        log.Printf("Error fetching events: %v", err)
        http.Error(w, "Error fetching events", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(events)
}

func (h *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    eventID := vars["id"]
    
    ctx := r.Context()
    event, err := h.eventService.GetEventByID(ctx, eventID)
    if err != nil {
        log.Printf("Error fetching event: %v", err)
        http.Error(w, "Event not found", http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(event)
}

func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
    var event models.Event
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    ctx := r.Context()
    if err := h.eventService.CreateEvent(ctx, &event); err != nil {
        log.Printf("Error creating event: %v", err)
        http.Error(w, "Error creating event", http.StatusInternalServerError)
        return
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

    ctx := r.Context()
    if err := h.eventService.UpdateEvent(ctx, &event); err != nil {
        log.Printf("Error updating event: %v", err)
        http.Error(w, "Error updating event", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(event)
}

func (h *EventHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    eventID := vars["id"]

    ctx := r.Context()
    if err := h.eventService.DeleteEvent(ctx, eventID); err != nil {
        log.Printf("Error deleting event: %v", err)
        http.Error(w, "Error deleting event", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}