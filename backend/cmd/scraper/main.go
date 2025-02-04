package main

import (
	"context"
	"log"
	"mma-scheduler/config"
	"mma-scheduler/pkg/databases"
	"mma-scheduler/pkg/scrapers"
)

func main() {
	log.Println("Starting scraper...")

	if err := config.LoadConfig("config/config.json"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	db := databases.GetDB()
	if db == nil {
		log.Fatal("Failed to initialize database connection")
	}
	defer db.Close(context.Background())

	scraper := scrapers.NewEventScraper(scrapers.DefaultConfig())
	events, err := scraper.ScrapeEvents()
	if err != nil {
		log.Fatal(err)
	}

	if err := databases.BatchInsertEvents(events); err != nil {
		log.Fatal(err)
	}

	log.Printf("Successfully scraped and stored %d events", len(events))
}
