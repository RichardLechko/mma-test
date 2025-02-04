package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"mma-scheduler/cache"
	"mma-scheduler/internal/models"

	"github.com/gorilla/mux"
)

type FightHandler struct {
	cache *cache.Cache
	// TODO: Add database service
}

func NewFightHandler(cache *cache.Cache) *FightHandler {
	return &FightHandler{
		cache: cache,
	}
}

// GetFights handles GET /fights
func (h *FightHandler) GetFights(w http.ResponseWriter, r *http.Request) {

	eventID := r.URL.Query().Get("event_id")
	weightClass := r.URL.Query().Get("weight_class")
	isTitleFight := r.URL.Query().Get("is_title_fight")

	if eventID != "" {
		ctx := r.Context()
		fights, err := h.cache.GetEventFights(ctx, eventID)
		if err != nil {
			log.Printf("Error fetching event fights: %v", err)
			// TODO: Fetch from database if not in cache
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fights)
		return
	}

	filters := map[string]interface{}{}
	if weightClass != "" {
		filters["weight_class"] = weightClass
	}
	if isTitleFight != "" {
		filters["is_title_fight"] = isTitleFight == "true"
	}

	log.Printf("Fetching fights with filters: %v", filters)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]models.Fight{})
}

// GetFight handles GET /fights/{id}
func (h *FightHandler) GetFight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fightID := vars["id"]

	ctx := r.Context()
	fight, err := h.cache.GetFight(ctx, fightID)
	if err != nil {
		log.Printf("Error fetching fight: %v", err)
		// TODO: Fetch from database if not in cache
		http.Error(w, "Fight not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fight)
}

// CreateFight handles POST /fights
func (h *FightHandler) CreateFight(w http.ResponseWriter, r *http.Request) {
	var fight models.Fight
	if err := json.NewDecoder(r.Body).Decode(&fight); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if fight.Fighter1ID == "" || fight.Fighter2ID == "" {
		http.Error(w, "Both fighters must be specified", http.StatusBadRequest)
		return
	}
	if fight.Fighter1ID == fight.Fighter2ID {
		http.Error(w, "Cannot create fight with same fighter", http.StatusBadRequest)
		return
	}

	// TODO: Save to database
	log.Printf("Creating new fight between fighters %s and %s",
		fight.Fighter1ID, fight.Fighter2ID)

	ctx := r.Context()
	if err := h.cache.SetFight(ctx, &fight); err != nil {
		log.Printf("Error caching fight: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fight)
}

// UpdateFight handles PUT /fights/{id}
func (h *FightHandler) UpdateFight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fightID := vars["id"]

	var fight models.Fight
	if err := json.NewDecoder(r.Body).Decode(&fight); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	fight.ID = fightID

	// TODO: Update in database
	log.Printf("Updating fight %s", fightID)

	ctx := r.Context()
	if err := h.cache.SetFight(ctx, &fight); err != nil {
		log.Printf("Error updating fight cache: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fight)
}

// DeleteFight handles DELETE /fights/{id}
func (h *FightHandler) DeleteFight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fightID := vars["id"]

	// TODO: Delete from database
	log.Printf("Deleting fight %s", fightID)

	ctx := r.Context()
	if err := h.cache.Delete(ctx, "fight:"+fightID); err != nil {
		log.Printf("Error removing fight from cache: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateFightResult handles POST /fights/{id}/result
func (h *FightHandler) UpdateFightResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fightID := vars["id"]

	var result models.FightResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Update fight result in database
	log.Printf("Updating result for fight %s", fightID)

	ctx := r.Context()
	if err := h.cache.Delete(ctx, "fight:"+fightID); err != nil {
		log.Printf("Error invalidating fight cache: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetFightWeighIns handles GET /fights/{id}/weigh-ins
func (h *FightHandler) GetFightWeighIns(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fightID := vars["id"]

	// TODO: Fetch weigh-in data from database
	log.Printf("Fetching weigh-in data for fight %s", fightID)

	weighIns := []models.WeighIn{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(weighIns)
}
