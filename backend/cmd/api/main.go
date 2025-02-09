package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gorilla/mux"
    "github.com/joho/godotenv"
    _ "github.com/lib/pq"
    
    "mma-scheduler/config"
    "mma-scheduler/internal/handlers"
    "mma-scheduler/internal/middleware"
    "mma-scheduler/internal/services"
)

func main() {
    if err := godotenv.Load("../../.env"); err != nil {
        log.Printf("Warning: .env file not found or error loading: %v", err)
    }

    if err := config.LoadConfig("../../config/config.json"); err != nil {
        log.Fatalf("Failed to load configuration: %v", err)
    }
    cfg := config.GetConfig()

    
    connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        cfg.Database.Host,
        cfg.Database.Port, 
        cfg.Database.User,
        cfg.Database.Password,
        cfg.Database.Database,  
    )

    sqlDB, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }

    db := services.NewDatabase(sqlDB)

    
    eventService := services.NewEventService(db.GetDB())  

    
    eventHandler := handlers.NewEventHandler(eventService)

    router := setupRouter(cfg, eventHandler)

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

func setupRouter(config *config.Config, eventHandler *handlers.EventHandler) *mux.Router {
    r := mux.NewRouter()

    r.Use(middleware.PanicRecovery)
    r.Use(middleware.RequestLogger)
    r.Use(middleware.CORS(middleware.CORSConfig{
        AllowedOrigins: []string{"http://localhost:3000"},
        MaxAge:         300,
    }))

    r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, "OK")
    }).Methods("GET")

    api := r.PathPrefix("/api/v1").Subrouter()

    
    api.HandleFunc("/events", eventHandler.GetEvents).Methods("GET")
    api.HandleFunc("/events/{id}", eventHandler.GetEvent).Methods("GET")
    api.HandleFunc("/events", eventHandler.CreateEvent).Methods("POST")
    api.HandleFunc("/events/{id}", eventHandler.UpdateEvent).Methods("PUT")
    api.HandleFunc("/events/{id}", eventHandler.DeleteEvent).Methods("DELETE")

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