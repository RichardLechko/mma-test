package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"mma-scheduler/config"
)

// ScraperJob represents a scraper to run
type ScraperJob struct {
	Name        string
	Description string
	Command     string
	Args        []string
}

func main() {
	// Set up logging with timestamps
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[MMA-Admin] ")
	log.Println("🚀 Starting Admin Job Runner")
	startTime := time.Now()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Load configuration
	if err := config.LoadConfig("config/config.json"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database (just for verification)
	db, err := connectToDatabase()
	if err != nil {
		log.Fatalf("Database connection error: %v", err)
	}
	defer db.Close()
	log.Println("✅ Database connection verified")

	// Define the scraper jobs in sequential order
	jobs := []ScraperJob{
		{
			Name:        "Event Scraper",
			Description: "Scrape events from UFC.com",
			Command:     "go",
			Args:        []string{"run", "cmd/scraper/main.go"},
		},
		{
			Name:        "Wiki Events Scraper",
			Description: "Scrape event information from Wikipedia",
			Command:     "go",
			Args:        []string{"run", "cmd/wiki_events/main.go"},
		},
		{
			Name:        "Fighters Scraper",
			Description: "Scrape fighter data from UFC.com",
			Command:     "go",
			Args:        []string{"run", "cmd/fighters/main.go"},
		},
		{
			Name:        "Wiki Fighters Scraper",
			Description: "Scrape fighter information from Wikipedia",
			Command:     "go",
			Args:        []string{"run", "cmd/wiki_fighters/main.go"},
		},
		{
			Name:        "Fights Scraper",
			Description: "Scrape fight data from UFC.com",
			Command:     "go",
			Args:        []string{"run", "cmd/fights/main.go"},
		},
	}

	// Run each job in sequence
	successCount := 0
	failCount := 0

	for i, job := range jobs {
		log.Printf("📋 Running job %d/%d: %s - %s", i+1, len(jobs), job.Name, job.Description)
		
		// Run the job
		err := runJob(job)
		
		if err != nil {
			log.Printf("❌ Job failed: %s - %v", job.Name, err)
			failCount++
		} else {
			log.Printf("✅ Job completed successfully: %s", job.Name)
			successCount++
		}
		
		// Small delay between jobs to ensure resources are freed
		time.Sleep(2 * time.Second)
	}

	// Print summary
	log.Printf("🏁 All jobs completed in %v", time.Since(startTime).Round(time.Second))
	log.Printf("📊 Results: %d succeeded, %d failed", successCount, failCount)
}

// connectToDatabase establishes a connection to the database
func connectToDatabase() (*sql.DB, error) {
	dbConfig := config.GetDatabaseConfig()

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=require&pool_max_conns=%d&pool_min_conns=%d&statement_timeout=60000",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Database,
		dbConfig.MaxOpenConns,
		dbConfig.MaxIdleConns,
	)

	// Open database connection
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Ping database to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// runJob executes a scraper job as a subprocess
func runJob(job ScraperJob) error {
	// Create command
	cmd := exec.Command(job.Command, job.Args...)
	
	// Set up pipe to capture output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Set environment variables
	cmd.Env = os.Environ()
	
	// Log the command being executed
	log.Printf("▶️ Executing: %s %v", job.Command, job.Args)
	
	// Run the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}
	
	return nil
}