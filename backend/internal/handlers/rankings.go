package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"mma-scheduler/cache"
	"mma-scheduler/internal/models"

	"github.com/gorilla/mux"
)

type RankingHandler struct {
	cache *cache.Cache
	// TODO: Add database service
}

func NewRankingHandler(cache *cache.Cache) *RankingHandler {
	return &RankingHandler{
		cache: cache,
	}
}

// GetRankings handles GET /rankings
func (h *RankingHandler) GetRankings(w http.ResponseWriter, r *http.Request) {
	promotionID := r.URL.Query().Get("promotion_id")
	weightClass := r.URL.Query().Get("weight_class")

	if promotionID == "" || weightClass == "" {
		http.Error(w, "promotion_id and weight_class are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	rankings, err := h.cache.GetWeightClassRankings(ctx, promotionID, weightClass)
	if err != nil {
		log.Printf("Error fetching rankings: %v", err)
		// TODO: Fetch from database if not in cache
		http.Error(w, "Error fetching rankings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rankings)
}

// GetRanking handles GET /rankings/{id}
func (h *RankingHandler) GetRanking(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	rankingID := vars["id"]

	ctx := r.Context()
	ranking, err := h.cache.GetRanking(ctx, rankingID)
	if err != nil {
		log.Printf("Error fetching ranking: %v", err)
		// TODO: Fetch from database if not in cache
		http.Error(w, "Ranking not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ranking)
}

// GetFighterRankings handles GET /fighters/{id}/rankings
func (h *RankingHandler) GetFighterRankings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fighterID := vars["id"]

	ctx := r.Context()
	rankings, err := h.cache.GetFighterRankings(ctx, fighterID)
	if err != nil {
		log.Printf("Error fetching fighter rankings: %v", err)
		// TODO: Fetch from database if not in cache
		rankings = []models.Ranking{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rankings)
}

// UpdateRankings handles PUT /rankings
func (h *RankingHandler) UpdateRankings(w http.ResponseWriter, r *http.Request) {
	var updates []models.RankingUpdate
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	promotionID := r.URL.Query().Get("promotion_id")
	weightClass := r.URL.Query().Get("weight_class")

	if promotionID == "" || weightClass == "" {
		http.Error(w, "promotion_id and weight_class are required", http.StatusBadRequest)
		return
	}

	// TODO: Update rankings in database
	log.Printf("Updating rankings for weight class %s in promotion %s", weightClass, promotionID)

	ctx := r.Context()
	cacheKey := fmt.Sprintf(cache.KeyWeightClassRankings, promotionID, weightClass)
	if err := h.cache.Delete(ctx, cacheKey); err != nil {
		log.Printf("Error invalidating rankings cache: %v", err)
	}

	for _, update := range updates {
		fighterKey := fmt.Sprintf(cache.KeyFighterRankings, update.FighterID)
		if err := h.cache.Delete(ctx, fighterKey); err != nil {
			log.Printf("Error invalidating fighter rankings cache: %v", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateRanking handles POST /rankings
func (h *RankingHandler) CreateRanking(w http.ResponseWriter, r *http.Request) {
	var ranking models.Ranking
	if err := json.NewDecoder(r.Body).Decode(&ranking); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if ranking.FighterID == "" || ranking.PromotionID == "" || ranking.WeightClass == "" {
		http.Error(w, "fighter_id, promotion_id, and weight_class are required", http.StatusBadRequest)
		return
	}

	// TODO: Save to database
	log.Printf("Creating new ranking for fighter %s in %s division",
		ranking.FighterID, ranking.WeightClass)

	ctx := r.Context()
	if err := h.cache.SetRanking(ctx, &ranking); err != nil {
		log.Printf("Error caching ranking: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ranking)
}
