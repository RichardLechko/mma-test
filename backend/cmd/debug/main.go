package main

import (
    "fmt"
    "log"
    "os"
    "strings"
    "mma-scheduler/pkg/scrapers"
)

// Custom scraper that wraps the original for debugging
type DebugScraper struct {
    originalScraper *scrapers.UFCFightScraper
}

// Override ScrapeFights method for debugging
func (d *DebugScraper) ScrapeFights(url string) ([]scrapers.UFCScrapedFight, error) {
    // Perform the original scrape
    fights, err := d.originalScraper.ScrapeFights(url)
    
    // Debug logging
    fmt.Println("\n🔍 Detailed Scraping Debug:")
    fmt.Println(strings.Repeat("-", 80))
    fmt.Printf("Total fights found: %d\n", len(fights))
    fmt.Println("\n📋 Raw Fight Element Inspection:")
    
    for i, fight := range fights {
        fmt.Printf("\n[Raw Fight %d]\n", i+1)
        
        fmt.Printf("  Fighter 1 Name Raw: %q\n", fight.Fighter1Name)
        fmt.Printf("  Fighter 2 Name Raw: %q\n", fight.Fighter2Name)
        fmt.Printf("  Given Name 1 Raw: %q\n", fight.Fighter1GivenName)
        fmt.Printf("  Given Name 2 Raw: %q\n", fight.Fighter2GivenName)
        fmt.Printf("  Last Name 1 Raw: %q\n", fight.Fighter1LastName)
        fmt.Printf("  Last Name 2 Raw: %q\n", fight.Fighter2LastName)
        fmt.Printf("  Weight Class Raw: %q\n", fight.WeightClass)
        fmt.Printf("  Is Title Fight: %v\n", fight.IsTitleFight)
        fmt.Printf("  Is Main Event: %v\n", fight.IsMainEvent)
        fmt.Printf("  Result 1: %q\n", fight.Fighter1Result)
        fmt.Printf("  Result 2: %q\n", fight.Fighter2Result)
        fmt.Printf("  Method: %q\n", fight.Method)
        fmt.Printf("  Round: %q\n", fight.Round)
        fmt.Printf("  Time: %q\n", fight.Time)
        
        
        // Add additional checks for filtering
        fmt.Printf("  Name Length Checks:\n")
        fmt.Printf("    Fighter 1 Name Empty: %v\n", fight.Fighter1Name == "")
        fmt.Printf("    Fighter 2 Name Empty: %v\n", fight.Fighter2Name == "")
        fmt.Printf("    Contains 'vs' in Fighter 1: %v\n", strings.Contains(fight.Fighter1Name, "vs"))
        fmt.Printf("    Contains 'vs' in Fighter 2: %v\n", strings.Contains(fight.Fighter2Name, "vs"))
    }
    
    return fights, err
}

func main() {
    // Check if URL is provided as argument
    if len(os.Args) < 2 {
        log.Fatalf("Please provide a UFC event URL as an argument")
    }
    eventURL := os.Args[1]

    // Create scraper
    scraperConfig := &scrapers.ScraperConfig{
        UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
    }
    originalScraper := scrapers.NewUFCFightScraper(scraperConfig)
    debugScraper := &DebugScraper{originalScraper}

    // Print debug info about scraper
    fmt.Println("🕵️ Scraper Configuration:")
    fmt.Printf("  User Agent: %s\n", scraperConfig.UserAgent)
    fmt.Printf("  Event URL: %s\n", eventURL)

    // Scrape fights
    ufcFights, err := debugScraper.ScrapeFights(eventURL)
    if err != nil {
        log.Fatalf("Failed to scrape fights: %v", err)
    }

    // Print detailed fight information
    fmt.Printf("🥊 Scraped %d fights from event: %s\n", len(ufcFights), eventURL)
    fmt.Println(strings.Repeat("=", 80))

    for i, fight := range ufcFights {
        fmt.Printf("\n[Fight %d]\n", i+1)
       
        // Detailed fight information logging
        fmt.Printf("🔹 Fighters: %s vs %s\n", fight.Fighter1Name, fight.Fighter2Name)
        fmt.Printf("  Weight Class: %s\n", fight.WeightClass)
       
        // Detailed fighter 1 information
        fmt.Printf("  Fighter 1:\n")
        fmt.Printf("    Full Name: %s\n", fight.Fighter1Name)
        fmt.Printf("    Given Name: %s\n", fight.Fighter1GivenName)
        fmt.Printf("    Last Name: %s\n", fight.Fighter1LastName)
        fmt.Printf("    Rank: %s\n", fight.Fighter1Rank)
        fmt.Printf("    Result: %s\n", fight.Fighter1Result)
       
        // Detailed fighter 2 information
        fmt.Printf("  Fighter 2:\n")
        fmt.Printf("    Full Name: %s\n", fight.Fighter2Name)
        fmt.Printf("    Given Name: %s\n", fight.Fighter2GivenName)
        fmt.Printf("    Last Name: %s\n", fight.Fighter2LastName)
        fmt.Printf("    Rank: %s\n", fight.Fighter2Rank)
        fmt.Printf("    Result: %s\n", fight.Fighter2Result)
       
        // Fight details
        fmt.Printf("  Fight Details:\n")
        fmt.Printf("    Main Event: %v\n", fight.IsMainEvent)
        fmt.Printf("    Title Fight: %v\n", fight.IsTitleFight)
        fmt.Printf("    Method: %s\n", fight.Method)
        fmt.Printf("    Round: %s\n", fight.Round)
        fmt.Printf("    Time: %s\n", fight.Time)
    }
}