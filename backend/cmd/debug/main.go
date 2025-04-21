package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/tebeka/selenium"
)

// DebugFight represents a fight scraped from UFC website
type DebugFight struct {
	Fighter1Name      string
	Fighter1GivenName string
	Fighter1LastName  string
	Fighter1Rank      string
	Fighter1Result    string
	Fighter2Name      string
	Fighter2GivenName string
	Fighter2LastName  string
	Fighter2Rank      string
	Fighter2Result    string
	WeightClass       string
	Method            string
	Round             string
	Time              string
	IsMainEvent       bool
	IsTitleFight      bool
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <url> [wait_seconds] [chromedriver_path]")
	}
	
	url := os.Args[1]
	
	// Parse wait time from command line or use default
	waitSeconds := 10 // Default wait time
	debugMode := false
	debugFightIndex := 8 // Default to 9th fight (index 8)
	chromedriverPath := "C:\\Users\\richa\\AppData\\Local\\Programs\\chromedriver.exe" // Default path
	
	// Parse additional arguments
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--debug" {
			debugMode = true
		} else if i < len(os.Args)-1 && os.Args[i] == "--fight" {
			if idx, err := strconv.Atoi(os.Args[i+1]); err == nil && idx > 0 {
				debugFightIndex = idx - 1 // Convert from 1-based to 0-based
				i++ // Skip the next argument
			}
		} else if os.Args[i] == "--chromedriver" && i < len(os.Args)-1 {
			chromedriverPath = os.Args[i+1]
			i++ // Skip the next argument
		} else if s, err := strconv.Atoi(os.Args[i]); err == nil {
			waitSeconds = s
		}
	}
	
	// Verify ChromeDriver exists
	if _, err := os.Stat(chromedriverPath); os.IsNotExist(err) {
		log.Fatalf("ChromeDriver not found at: %s", chromedriverPath)
	}
	
	fmt.Printf("Setting up ChromeDriver at %s to navigate to %s\n", chromedriverPath, url)
	
	// Set up ChromeDriver capabilities with Chrome options
	caps := selenium.Capabilities{
		"browserName": "chrome",
	}
	
	// Add Chrome-specific options
	chromeOpts := map[string]interface{}{
		"args": []string{
			"--headless",
			"--no-sandbox",
			"--disable-dev-shm-usage",
		},
	}
	
	// Add Chrome options to capabilities
	caps["goog:chromeOptions"] = chromeOpts
	
	// Start a ChromeDriver instance with explicit path
	service, err := selenium.NewChromeDriverService(chromedriverPath, 4444)
	if err != nil {
		log.Fatalf("Error starting ChromeDriver service: %v", err)
	}
	defer service.Stop()
	
	// Connect to the WebDriver instance
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", 4444))
	if err != nil {
		log.Fatalf("Error connecting to ChromeDriver: %v", err)
	}
	defer wd.Quit()
	
	// Navigate to the URL
	fmt.Printf("Navigating to %s and waiting %d seconds for content to load...\n", url, waitSeconds)
	if err := wd.Get(url); err != nil {
		log.Fatalf("Error navigating to URL: %v", err)
	}
	
	// Wait for page to load
	fmt.Printf("Page loaded, waiting %d seconds for JavaScript execution...\n", waitSeconds)
	time.Sleep(time.Duration(waitSeconds) * time.Second)
	
	// Get the page source after JavaScript has executed
	htmlContent, err := wd.PageSource()
	if err != nil {
		log.Fatalf("Error getting page source: %v", err)
	}
	
	if len(htmlContent) == 0 {
		log.Fatal("Failed to retrieve HTML content from page")
	}
	
	// Save the HTML content to a file for debugging
	if err := ioutil.WriteFile("ufc_page_loaded.html", []byte(htmlContent), 0644); err != nil {
		log.Printf("Warning: Failed to save HTML: %v", err)
	} else {
		fmt.Println("Loaded HTML saved to ufc_page_loaded.html")
	}
	
	// Parse the HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		log.Fatalf("Failed to parse HTML: %v", err)
	}
	
	// Get event info
	eventTitle := cleanText(doc.Find(".c-hero__headline").Text())
	eventDate := cleanText(doc.Find(".c-hero__headline-suffix").Text())
	
	fmt.Printf("EVENT: %s\n", eventTitle)
	fmt.Printf("DATE: %s\n", eventDate)
	fmt.Printf("URL: %s\n\n", url)
	
	// Find all fight elements
	fightElements := doc.Find(".c-listing-fight")
	
	// Check if we found any fights
	fightCount := fightElements.Length()
	if fightCount == 0 {
		log.Fatal("No fights found on the page. The page might not have loaded completely.")
	}
	
	fmt.Printf("Found %d fights on the page.\n\n", fightCount)
	
	// Process fights
	fightElements.Each(func(i int, fightElement *goquery.Selection) {
		fight := extractFightInfo(i, fightElement)
		
		if fight != nil && fight.Fighter1Name != "" && fight.Fighter2Name != "" {
			// Display fight information
			fmt.Printf("FIGHT #%d: %s vs %s (%s)\n",
				i+1,
				fight.Fighter1Name,
				fight.Fighter2Name,
				fight.WeightClass)
			
			fmt.Printf("  RED: %s (%s) - Result: %s\n",
				fight.Fighter1Name,
				fight.Fighter1Rank,
				fight.Fighter1Result)
			
			fmt.Printf("  BLUE: %s (%s) - Result: %s\n",
				fight.Fighter2Name,
				fight.Fighter2Rank,
				fight.Fighter2Result)
			
			if fight.Method != "" {
				fmt.Printf("  RESULT: %s - Round %s at %s\n",
					fight.Method,
					fight.Round,
					fight.Time)
			} else {
				fmt.Println("  RESULT: Not determined")
			}
			
			fmt.Println()
		}
	})
	
	// Debug mode - display HTML of a specific fight for debugging
	if debugMode && fightElements.Length() > debugFightIndex {
		problemFight := fightElements.Eq(debugFightIndex)
		
		// Debug red corner
		redCorner := problemFight.Find(".c-listing-fight__corner--red")
		redOutcome := redCorner.Find(".c-listing-fight__outcome-wrapper").Text()
		redWin := redCorner.Find(".c-listing-fight__outcome--win").Length()
		
		fmt.Printf("\nDEBUG FIGHT #%d RED CORNER:\n", debugFightIndex+1)
		fmt.Printf("  Outcome wrapper text: %s\n", cleanText(redOutcome))
		fmt.Printf("  Win element found: %v\n", redWin > 0)
		
		// Debug HTML structure
		html, _ := redCorner.Html()
		fmt.Println("\nRED CORNER HTML:")
		fmt.Println(html)
		
		// Also debug the blue corner
		blueCorner := problemFight.Find(".c-listing-fight__corner--blue")
		blueOutcome := blueCorner.Find(".c-listing-fight__outcome-wrapper").Text()
		blueWin := blueCorner.Find(".c-listing-fight__outcome--win").Length()
		
		fmt.Printf("\nDEBUG FIGHT #%d BLUE CORNER:\n", debugFightIndex+1)
		fmt.Printf("  Outcome wrapper text: %s\n", cleanText(blueOutcome))
		fmt.Printf("  Win element found: %v\n", blueWin > 0)
		
		// Debug HTML structure
		blueHtml, _ := blueCorner.Html()
		fmt.Println("\nBLUE CORNER HTML:")
		fmt.Println(blueHtml)
	}
}

func extractFightInfo(index int, fightElement *goquery.Selection) *DebugFight {
	// Initialize the fight
	fight := &DebugFight{}
	
	// Weight class
	weightClass := fightElement.Find(".c-listing-fight__class-text").First().Text()
	fight.WeightClass = cleanText(weightClass)
	
	// Check if title fight
	fight.IsTitleFight = strings.Contains(strings.ToLower(weightClass), "title")
	
	// Main event is typically the first fight
	fight.IsMainEvent = index == 0
	
	// Find corner names
	cornerNames := fightElement.Find(".c-listing-fight__corner-name")
	
	// Process first fighter
	if cornerNames.Length() > 0 {
		redCorner := cornerNames.Eq(0)
		givenName := cleanText(redCorner.Find(".c-listing-fight__corner-given-name").Text())
		familyName := cleanText(redCorner.Find(".c-listing-fight__corner-family-name").Text())
		
		// Fallback if given/family name extraction fails
		if givenName == "" || familyName == "" {
			fullName := cleanText(redCorner.Text())
			nameParts := strings.Fields(fullName)
			if len(nameParts) > 0 {
				givenName = nameParts[0]
				familyName = strings.Join(nameParts[1:], " ")
			}
		}
		
		fight.Fighter1GivenName = givenName
		fight.Fighter1LastName = familyName
		fight.Fighter1Name = strings.TrimSpace(givenName + " " + familyName)
	}
	
	// Process second fighter
	if cornerNames.Length() > 1 {
		blueCorner := cornerNames.Eq(1)
		givenName := cleanText(blueCorner.Find(".c-listing-fight__corner-given-name").Text())
		familyName := cleanText(blueCorner.Find(".c-listing-fight__corner-family-name").Text())
		
		// Fallback if given/family name extraction fails
		if givenName == "" || familyName == "" {
			fullName := cleanText(blueCorner.Text())
			nameParts := strings.Fields(fullName)
			if len(nameParts) > 0 {
				givenName = nameParts[0]
				familyName = strings.Join(nameParts[1:], " ")
			}
		}
		
		fight.Fighter2GivenName = givenName
		fight.Fighter2LastName = familyName
		fight.Fighter2Name = strings.TrimSpace(givenName + " " + familyName)
	}
	
	// Check for special result outcomes first (Draw, No Contest)
	redDrawElement := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome--draw")
	blueDrawElement := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome--draw")
	redNoContestElement := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome--no-contest")
	blueNoContestElement := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome--no-contest")
	
	// Check for Draw
	if (redDrawElement.Length() > 0 && strings.Contains(redDrawElement.Text(), "Draw")) ||
	   (blueDrawElement.Length() > 0 && strings.Contains(blueDrawElement.Text(), "Draw")) {
		fight.Fighter1Result = "Draw"
		fight.Fighter2Result = "Draw"
		// Set method to Draw as well to indicate the fight result
		fight.Method = "Draw"
	} else if (redNoContestElement.Length() > 0) || (blueNoContestElement.Length() > 0) {
		// Check for No Contest
		fight.Fighter1Result = "No Contest"
		fight.Fighter2Result = "No Contest"
		// Set method to No Contest as well
		fight.Method = "No Contest"
	} else {
		// Extract results for red corner (fighter 1) if not a special result
		redWinElement := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome--win")
		if redWinElement.Length() > 0 {
			fight.Fighter1Result = "Win"
		} else {
			redOutcome := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__outcome").Text()
			if redOutcome != "" {
				fight.Fighter1Result = cleanText(redOutcome)
			}
		}
		
		// Extract results for blue corner (fighter 2) if not a special result
		blueWinElement := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome--win")
		if blueWinElement.Length() > 0 {
			fight.Fighter2Result = "Win"
		} else {
			blueOutcome := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__outcome").Text()
			if blueOutcome != "" {
				fight.Fighter2Result = cleanText(blueOutcome)
			}
		}
		
		// If we still don't have results, try to infer from the method
		if fight.Fighter1Result == "" && fight.Fighter2Result == "" {
			method := fightElement.Find(".c-listing-fight__result-text.method").Text()
			
			// Check if the method text indicates a draw or no contest
			methodLower := strings.ToLower(method)
			if strings.Contains(methodLower, "draw") {
				fight.Fighter1Result = "Draw"
				fight.Fighter2Result = "Draw"
				fight.Method = "Draw"
			} else if strings.Contains(methodLower, "no contest") || strings.Contains(methodLower, "nc") {
				fight.Fighter1Result = "No Contest"
				fight.Fighter2Result = "No Contest"
				fight.Method = "No Contest"
			} else if method != "" {
				// Look for special case - banner or award that might indicate winner
				redBanner := fightElement.Find(".c-listing-fight__corner-body--red .c-listing-fight__banner")
				blueBanner := fightElement.Find(".c-listing-fight__corner-body--blue .c-listing-fight__banner")
				
				if redBanner.Length() > 0 && strings.Contains(strings.ToLower(redBanner.Text()), "win") {
					fight.Fighter1Result = "Win"
				} else if blueBanner.Length() > 0 && strings.Contains(strings.ToLower(blueBanner.Text()), "win") {
					fight.Fighter2Result = "Win"
				}
			}
		}
	}
	
	// Get fighter ranks
	ranks := fightElement.Find(".c-listing-fight__corner-rank")
	if ranks.Length() >= 2 {
		// First rank is for red corner, second for blue corner
		fight.Fighter1Rank = cleanText(ranks.Eq(0).Text())
		fight.Fighter2Rank = cleanText(ranks.Eq(1).Text())
	} else {
		// Try alternate rank selectors if the primary ones don't work
		redRank := fightElement.Find(".js-listing-fight__corner-rank:first-child span").Text()
		blueRank := fightElement.Find(".js-listing-fight__corner-rank:last-child span").Text()
		
		if redRank != "" {
			fight.Fighter1Rank = cleanText(redRank)
		}
		if blueRank != "" {
			fight.Fighter2Rank = cleanText(blueRank)
		}
	}
	
	// Fight results
	roundText := fightElement.Find(".c-listing-fight__result-text.round").Text()
	timeText := fightElement.Find(".c-listing-fight__result-text.time").Text()
	methodText := fightElement.Find(".c-listing-fight__result-text.method").Text()
	
	// Fix duplicated text
	fight.Round = fixDuplicatedText(cleanText(roundText))
	fight.Time = fixDuplicatedText(cleanText(timeText))
	
	// Only set Method if not already set as a special result
	if fight.Method == "" {
		fight.Method = fixDuplicatedText(cleanText(methodText))
	}
	
	return fight
}

// cleanText removes extra whitespace and normalizes text
func cleanText(text string) string {
	// Remove newlines and tabs
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	
	// Replace multiple spaces with a single space
	text = strings.Join(strings.Fields(text), " ")
	
	return strings.TrimSpace(text)
}

// fixDuplicatedText fixes issues with duplicated text in scraped content
func fixDuplicatedText(text string) string {
	if len(text) == 0 {
		return text
	}
	
	// Check for exact duplication (same text repeated)
	halfLen := len(text) / 2
	if halfLen > 0 && text[:halfLen] == text[halfLen:] {
		return text[:halfLen]
	}
	
	// Try a more sophisticated approach for pattern detection
	for i := 1; i <= len(text)/2; i++ {
		if i <= 0 {
			continue
		}
		
		pattern := text[:i]
		repeats := true
		
		for j := i; j < len(text); j += i {
			end := j + i
			if end > len(text) {
				end = len(text)
			}
			
			if text[j:end] != pattern[:end-j] {
				repeats = false
				break
			}
		}
		
		if repeats {
			return pattern
		}
	}
	
	return text
}

// extractRankNumber converts rank strings like "#1", "#C", "C" to numeric values
func extractRankNumber(rank string) int {
	rank = strings.TrimSpace(strings.ToLower(rank))
	
	if rank == "" {
		return 100 // Unranked
	}
	
	// Check for champion
	if strings.Contains(rank, "c") {
		return 0
	}
	
	// Extract numeric part
	re := regexp.MustCompile(`#?(\d+)`)
	matches := re.FindStringSubmatch(rank)
	if len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}
	
	return 100 // Default to unranked
}