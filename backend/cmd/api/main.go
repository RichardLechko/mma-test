package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"mma-scheduler/cache"
	"mma-scheduler/config"
	"mma-scheduler/internal/handlers"
	"mma-scheduler/internal/middleware"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		log.Printf("Warning: .env file not found or error loading: %v", err)
	}

	cfg := config.GetConfig()

	cacheClient, err := initCache(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}
	defer cacheClient.Close()

	cacheWrapper, err := cache.NewCache(cache.CacheConfig{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Fatalf("Failed to initialize cache wrapper: %v", err)
	}

	router := setupRouter(cfg, cacheWrapper)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	setupGracefulShutdown(server)

	log.Printf("Server is running on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func setupRouter(config *config.Config, cache *cache.Cache) *mux.Router {
	r := mux.NewRouter()

	r.Use(middleware.PanicRecovery)
	r.Use(middleware.RequestLogger)
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000"},
		MaxAge:         300,
	}))

	fighterHandler := handlers.NewFighterHandler(cache)
	eventHandler := handlers.NewEventHandler(cache)
	fightHandler := handlers.NewFightHandler(cache)
	rankingHandler := handlers.NewRankingHandler(cache)

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}).Methods("GET")

	api := r.PathPrefix("/api").Subrouter()

	protected := api.PathPrefix("/v1").Subrouter()
	protected.Use(middleware.Auth(middleware.AuthConfig{
		JWTSecret: config.JWT.Secret,
	}))

	protected.HandleFunc("/fighters", fighterHandler.GetFighters).Methods("GET")
	protected.HandleFunc("/fighters/{id}", fighterHandler.GetFighter).Methods("GET")
	protected.HandleFunc("/fighters", fighterHandler.CreateFighter).Methods("POST")
	protected.HandleFunc("/fighters/{id}", fighterHandler.UpdateFighter).Methods("PUT")
	protected.HandleFunc("/fighters/{id}", fighterHandler.DeleteFighter).Methods("DELETE")
	protected.HandleFunc("/fighters/{id}/record", fighterHandler.GetFighterRecord).Methods("GET")
	protected.HandleFunc("/fighters/{id}/upcoming", fighterHandler.GetFighterUpcomingFights).Methods("GET")

	protected.HandleFunc("/events", eventHandler.GetEvents).Methods("GET")
	protected.HandleFunc("/events/{id}", eventHandler.GetEvent).Methods("GET")
	protected.HandleFunc("/events", eventHandler.CreateEvent).Methods("POST")
	protected.HandleFunc("/events/{id}", eventHandler.UpdateEvent).Methods("PUT")
	protected.HandleFunc("/events/{id}", eventHandler.DeleteEvent).Methods("DELETE")

	protected.HandleFunc("/fights", fightHandler.GetFights).Methods("GET")
	protected.HandleFunc("/fights/{id}", fightHandler.GetFight).Methods("GET")
	protected.HandleFunc("/fights", fightHandler.CreateFight).Methods("POST")
	protected.HandleFunc("/fights/{id}", fightHandler.UpdateFight).Methods("PUT")
	protected.HandleFunc("/fights/{id}", fightHandler.DeleteFight).Methods("DELETE")
	protected.HandleFunc("/fights/{id}/result", fightHandler.UpdateFightResult).Methods("POST")
	protected.HandleFunc("/fights/{id}/weigh-ins", fightHandler.GetFightWeighIns).Methods("GET")

	protected.HandleFunc("/rankings", rankingHandler.GetRankings).Methods("GET")
	protected.HandleFunc("/rankings/{id}", rankingHandler.GetRanking).Methods("GET")
	protected.HandleFunc("/rankings", rankingHandler.CreateRanking).Methods("POST")
	protected.HandleFunc("/rankings", rankingHandler.UpdateRankings).Methods("PUT")

	admin := protected.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.RequireRole("admin"))

	return r
}

func setupGracefulShutdown(server *http.Server) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Error during server shutdown: %v", err)
		}
	}()
}

func initCache(config config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	return client, nil
}
