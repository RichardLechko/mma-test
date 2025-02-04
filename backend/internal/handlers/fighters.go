package handlers

import (
	"encoding/json"
	"log"
	"mma-scheduler/cache"
	"mma-scheduler/internal/models"
	"net/http"

	"github.com/gorilla/mux"
)

type FighterHandler struct {
	cache *cache.Cache
	// TODO: Add database service
}

func NewFighterHandler(cache *cache.Cache) *FighterHandler {
	return &FighterHandler{
		cache: cache,
	}
}

// GetFighters handles GET /fighters
func (h *FighterHandler) GetFighters(w http.ResponseWriter, r *http.Request) {
	weightClass := r.URL.Query().Get("weight_class")
	active := r.URL.Query().Get("active")

	filters := map[string]interface{}{}
	if weightClass != "" {
		filters["weight_class"] = weightClass
	}
	if active != "" {
		filters["active"] = active == "true"
	}

	// TODO: Use filters in database query

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]models.Fighter{})
}

func (h *FighterHandler) GetFighter(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fighterID := vars["id"]

	ctx := r.Context()
	fighter, err := h.cache.GetFighter(ctx, fighterID)
	if err != nil {
		// TODO: Fetch from database if not in cache
		log.Printf("Error fetching fighter: %v", err)
		http.Error(w, "Fighter not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fighter)
}

func (h *FighterHandler) CreateFighter(w http.ResponseWriter, r *http.Request) {
	var fighter models.Fighter
	if err := json.NewDecoder(r.Body).Decode(&fighter); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if fighter.FullName == "" {
		http.Error(w, "Full name is required", http.StatusBadRequest)
		return
	}

	// TODO: Save to database

	ctx := r.Context()
	if err := h.cache.SetFighter(ctx, &fighter); err != nil {
		log.Printf("Error caching fighter: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fighter)
}

// UpdateFighter handles PUT /fighters/{id}
func (h *FighterHandler) UpdateFighter(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fighterID := vars["id"]

	var fighter models.Fighter
	if err := json.NewDecoder(r.Body).Decode(&fighter); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	fighter.ID = fighterID

	// TODO: Update in database

	ctx := r.Context()
	if err := h.cache.SetFighter(ctx, &fighter); err != nil {
		log.Printf("Error updating fighter cache: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fighter)
}

func (h *FighterHandler) DeleteFighter(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fighterID := vars["id"]

	// TODO: Delete from database

	ctx := r.Context()
	if err := h.cache.Delete(ctx, "fighter:"+fighterID); err != nil {
		log.Printf("Error removing fighter from cache: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FighterHandler) GetFighterRecord(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fighterID := vars["id"]

	log.Printf("Fetching record for fighter ID: %s", fighterID)

	// TODO: Fetch fighter record from database using fighterID
	record := models.Record{
		Wins:       0,
		Losses:     0,
		Draws:      0,
		NoContests: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

func (h *FighterHandler) GetFighterUpcomingFights(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fighterID := vars["id"]

	log.Printf("Fetching upcoming fights for fighter ID: %s", fighterID)

	// TODO: Fetch upcoming fights from database using fighterID

	fights := []models.Fight{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fights)
}
