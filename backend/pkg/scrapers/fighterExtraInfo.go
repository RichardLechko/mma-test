package scrapers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type FighterExtraInfo struct {
	KOLosses      int
	SubLosses     int
	DecLosses     int
	DQLosses      int
	NoContests    int
	FightingOutOf string
	Age           int
	WikiURL       string

	KOWins  int
	SubWins int
	DecWins int
}

type WikiFighterScraper struct {
	config       *ScraperConfig
	client       *http.Client
	clientPool   []*http.Client
	poolMutex    sync.Mutex
	rateLimiter  <-chan time.Time
	urlAttempts  int
	proxyEnabled bool
}

func NewWikiFighterScraper(config *ScraperConfig) *WikiFighterScraper {
	// Number of concurrent HTTP clients to maintain
	const clientPoolSize = 8

	// Create a pool of HTTP clients
	clientPool := make([]*http.Client, clientPoolSize)
	for i := 0; i < clientPoolSize; i++ {
		clientPool[i] = &http.Client{
			Timeout: 15 * time.Second, // Slightly longer timeout for reliability
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
			},
		}
	}

	// Create a rate limiter to avoid overwhelming Wikipedia
	// 3 requests per second is generally safe
	rateLimiter := time.Tick(333 * time.Millisecond)

	return &WikiFighterScraper{
		config:      config,
		client:      clientPool[0], // Fallback client
		clientPool:  clientPool,
		urlAttempts: 3,
		rateLimiter: rateLimiter,
	}
}

// getClient returns an HTTP client from the pool
func (s *WikiFighterScraper) getClient() *http.Client {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	// Simple round-robin from the client pool
	client := s.clientPool[0]
	// Rotate the pool
	s.clientPool = append(s.clientPool[1:], s.clientPool[0])
	return client
}

// fetchURL attempts to fetch and validate a URL with concurrency control
func (s *WikiFighterScraper) fetchURL(ctx context.Context, url string, fighterName string) (*goquery.Document, error) {
	if url == "" {
		return nil, fmt.Errorf("empty URL")
	}

	// Respect rate limiting
	select {
	case <-s.rateLimiter:
		// Continue after rate limit delay
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Get a client from the pool
	client := s.getClient()

	// Create the request with context for timeout/cancellation
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.config.UserAgent)

	// Execute the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	// Parse the HTML document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check if this is a disambiguation page
	if isDisambiguationPage(doc) {
		return nil, fmt.Errorf("disambiguation page")
	}

	// Verify it's a fighter page
	if evidenceScore := isFighterPage(doc, fighterName); evidenceScore < 2 {
		return nil, fmt.Errorf("not a fighter page (score: %d)", evidenceScore)
	}

	return doc, nil
}

// isDisambiguationPage checks if the page is a Wikipedia disambiguation page
func isDisambiguationPage(doc *goquery.Document) bool {
	// Method 1: Check for the "disambigbox" template
	if doc.Find(".disambigbox").Length() > 0 {
		return true
	}

	// Method 2: Check for "may refer to" text in first paragraph
	firstPara := doc.Find(".mw-parser-output > p").First().Text()
	if strings.Contains(strings.ToLower(firstPara), "may refer to") ||
		strings.Contains(strings.ToLower(firstPara), "commonly refers to") ||
		strings.Contains(strings.ToLower(firstPara), "disambiguation") {
		return true
	}

	// Method 3: Check for dmbox class
	if doc.Find(".dmbox").Length() > 0 {
		return true
	}

	return false
}

// isFighterPage calculates an evidence score for whether this is an MMA fighter page
func isFighterPage(doc *goquery.Document, fighterName string) int {
	evidenceScore := 0
	pageTitle := doc.Find("h1#firstHeading").Text()

	// Check page title - does it contain the fighter's name?
	if strings.Contains(strings.ToLower(pageTitle), strings.ToLower(fighterName)) {
		evidenceScore += 2
	}

	// Check for MMA-related links
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		if strings.Contains(href, "/wiki/Ultimate_Fighting_Championship") ||
			strings.Contains(href, "/wiki/Mixed_martial_arts") ||
			strings.Contains(href, "/wiki/Featherweight_(MMA)") ||
			strings.Contains(href, "/wiki/Lightweight_(MMA)") ||
			strings.Contains(href, "/wiki/Welterweight_(MMA)") ||
			strings.Contains(href, "/wiki/Middleweight_(MMA)") ||
			strings.Contains(href, "/wiki/Light_Heavyweight_(MMA)") ||
			strings.Contains(href, "/wiki/Heavyweight_(MMA)") ||
			strings.Contains(href, "/wiki/Bantamweight_(MMA)") ||
			strings.Contains(href, "/wiki/Flyweight_(MMA)") {
			evidenceScore += 3
		}
	})

	// Check categories
	doc.Find(".mw-normal-catlinks ul li a").Each(func(i int, s *goquery.Selection) {
		category := strings.ToLower(s.Text())
		if strings.Contains(category, "mixed martial artists") ||
			strings.Contains(category, "ufc") ||
			strings.Contains(category, "ultimate fighting championship") {
			evidenceScore += 3
		}
	})

	// Check infobox
	doc.Find("table.infobox th, table.infobox td").Each(func(i int, s *goquery.Selection) {
		text := strings.ToLower(s.Text())

		if strings.Contains(text, "mma record") ||
			strings.Contains(text, "fight record") ||
			strings.Contains(text, "ufc") ||
			strings.Contains(text, "weight class") ||
			strings.Contains(text, "team") ||
			strings.Contains(text, "trainer") ||
			strings.Contains(text, "wrestling") ||
			strings.Contains(text, "boxing") ||
			strings.Contains(text, "martial art") {
			evidenceScore += 2
		}
	})

	// Check full text for MMA-related keywords
	fullText := doc.Text()
	lowerFullText := strings.ToLower(fullText)

	if strings.Contains(lowerFullText, "ufc") {
		evidenceScore += 2
	}
	if strings.Contains(lowerFullText, "mixed martial artist") {
		evidenceScore += 3
	}
	if strings.Contains(lowerFullText, "professional mixed martial artist") {
		evidenceScore += 4
	}
	if strings.Contains(lowerFullText, "ultimate fighting championship") {
		evidenceScore += 3
	}

	return evidenceScore
}

func (s *WikiFighterScraper) ScrapeExtraInfo(fighterName, wikiURL, ufcURL string, ufcWins, ufcLosses int) (*FighterExtraInfo, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Prepare alternative URLs to try in parallel
	cleanName := strings.ReplaceAll(fighterName, " ", "_")
	cleanName = strings.ReplaceAll(cleanName, ".", "") // Remove periods

	// Instead of removing apostrophes, encode them properly for URLs
	// Replace apostrophes with %27 for URL encoding
	encodedName := strings.ReplaceAll(cleanName, "'", "%27")

	// Collection of URLs to try
	allURLs := []string{}

	// Add the original URL only if it exists and isn't NULL
	if wikiURL != "" && wikiURL != "NULL" {
		allURLs = append(allURLs, wikiURL)
	}

	// Add generated URLs
	allURLs = append(allURLs,
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s", encodedName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(fighter)", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(fighter)", encodedName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(mixed_martial_artist)", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(mixed_martial_artist)", encodedName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(martial_artist)", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(martial_artist)", encodedName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(MMA)", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(MMA)", encodedName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(UFC)", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(UFC)", encodedName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(The_Ultimate_Fighter)", cleanName))

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Create a channel to receive successful results with their URLs
	type docResult struct {
		doc *goquery.Document
		url string
	}
	resultChan := make(chan docResult, len(allURLs))

	// Create a context with cancellation for early termination
	fetchCtx, fetchCancel := context.WithCancel(ctx)
	defer fetchCancel()

	// Launch concurrent requests to all URLs
	for _, url := range allURLs {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// Try to fetch and validate the URL
			doc, err := s.fetchURL(fetchCtx, url, fighterName)
			if err == nil && doc != nil {
				// Successfully found a valid page, send it to the result channel with its URL
				select {
				case resultChan <- docResult{doc: doc, url: url}:
					// Cancel other requests since we found a valid result
					fetchCancel()
				default:
					// Channel is full, which means we already have a result
				}
			}
		}(url)
	}

	// Use a goroutine to close the result channel when all URL fetches are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Wait for a successful result or for all goroutines to finish
	var doc *goquery.Document
	var successfulURL string
	select {
	case result := <-resultChan:
		doc = result.doc
		successfulURL = result.url
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout while fetching Wikipedia page for %s", fighterName)
	}

	// If we didn't get a valid document, return an error
	if doc == nil {
		return nil, fmt.Errorf("failed to find a valid Wikipedia page for %s", fighterName)
	}

	// Use goroutines to extract different types of information in parallel
	var wgExtract sync.WaitGroup
	info := &FighterExtraInfo{
		WikiURL: successfulURL, // Store the successful URL
	}

	// Extract fighting out of information
	wgExtract.Add(1)
	go func() {
		defer wgExtract.Done()
		info.FightingOutOf = extractFightingOutOf(doc)
	}()

	// Extract record data
	wgExtract.Add(1)
	go func() {
		defer wgExtract.Done()
		extractRecordData(doc, info)
	}()

	// Extract age information
	wgExtract.Add(1)
	go func() {
		defer wgExtract.Done()
		info.Age = extractAge(doc)
	}()

	// Extract no contests directly (not as part of record data)
	wgExtract.Add(1)
	go func() {
		defer wgExtract.Done()
		extractNoContests(doc, info)
	}()

	// Wait for all extraction goroutines to complete
	wgExtract.Wait()

	// Final validation
	validateFighterInfo(info, ufcWins, ufcLosses)

	// Log what we found
	fmt.Printf("Fighter %s: Found FightingOutOf=%s, Age=%d, NoContests=%d, Win methods: KO=%d, Sub=%d, Dec=%d\n",
		fighterName, info.FightingOutOf, info.Age, info.NoContests, info.KOWins, info.SubWins, info.DecWins)

	// Make sure we're not nullifying the WikiURL even if other fields are empty
	if isEmpty(info) && info.WikiURL != "" {
		// If we have a valid URL but no other data, still return the info
		return info, nil
	} else if isEmpty(info) {
		return nil, nil
	}

	return info, nil
}

// isEmpty checks if all fight data fields are empty
func isEmpty(info *FighterExtraInfo) bool {
	return info.KOLosses == 0 &&
		info.SubLosses == 0 &&
		info.DecLosses == 0 &&
		info.DQLosses == 0 &&
		info.NoContests == 0 &&
		info.FightingOutOf == "" &&
		info.KOWins == 0 &&
		info.SubWins == 0 &&
		info.DecWins == 0 &&
		info.Age == 0
	// Not checking WikiURL as we want to return the object even if only WikiURL is set
}

// Extract the "Fighting out of" information as a JSON array string
func extractFightingOutOf(doc *goquery.Document) string {
	var locations []string

	// Look for the "Fighting out of" row
	doc.Find("tr").Each(func(i int, row *goquery.Selection) {
		headerCell := row.Find("th").First()
		if headerCell.Length() == 0 {
			return
		}

		headerText := strings.ToLower(strings.TrimSpace(headerCell.Text()))
		if strings.Contains(headerText, "fighting out of") {
			dataCell := row.Find("td").First()
			if dataCell.Length() > 0 {
				// First, try to handle lists with <li> elements
				listItems := dataCell.Find("li")
				if listItems.Length() > 0 {
					// Handle structured list
					listItems.Each(func(i int, item *goquery.Selection) {
						locationText := sanitizeLocationText(item.Text())
						if locationText != "" {
							locations = append(locations, locationText)
						}
					})
				} else {
					// For cells with <br> tags, we need to process each element separately
					// Clone the cell to avoid modifying the original
					cellContents := dataCell.Clone()

					// Replace <br> tags with a special marker
					html, _ := cellContents.Html()
					html = strings.ReplaceAll(html, "<br>", "|||BREAK|||")
					html = strings.ReplaceAll(html, "<br/>", "|||BREAK|||")
					html = strings.ReplaceAll(html, "<br />", "|||BREAK|||")

					// Create a new document with the modified HTML
					tempDoc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + html + "</div>"))
					if err == nil {
						// Get the text with our special markers
						fullText := tempDoc.Find("div").Text()

						// Split by our special marker
						parts := strings.Split(fullText, "|||BREAK|||")

						for _, part := range parts {
							locationText := sanitizeLocationText(part)
							if locationText != "" {
								locations = append(locations, locationText)
							}
						}
					}
				}
			}
		}
	})

	// If we have locations, format them as a JSON array string
	if len(locations) > 0 {
		// Format as {loc1}, {loc2}, {loc3}
		var formattedLocations []string
		for _, loc := range locations {
			formattedLocations = append(formattedLocations, "{"+loc+"}")
		}
		return strings.Join(formattedLocations, ", ")
	}

	return ""
}

// Helper function to sanitize location text
func sanitizeLocationText(text string) string {
	// Remove CSS styles that might be part of the content
	styleRegex := regexp.MustCompile(`\.mw-parser-output[^}]+}`)
	text = styleRegex.ReplaceAllString(text, "")

	// Remove citations like [1], [2], etc.
	citationRegex := regexp.MustCompile(`\[\d+\]`)
	text = citationRegex.ReplaceAllString(text, "")

	// Remove extra whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func extractRecordData(doc *goquery.Document, info *FighterExtraInfo) {
	var inWinsSection, inLossesSection bool
	var totalWins, totalLosses int

	// Process each row in the infobox table
	doc.Find("table.infobox tr, table.vcard tr").Each(func(i int, row *goquery.Selection) {
		// Extract header text - we need to be careful with HTML entities like &nbsp;
		headerCell := row.Find("th").First()
		if headerCell.Length() == 0 {
			return
		}

		// Get both text and HTML of the header for different matching approaches
		headerText := strings.TrimSpace(headerCell.Text())
		headerHTML, _ := headerCell.Html()
		headerTextLower := strings.ToLower(headerText)

		// Check for major section headers
		if (strings.Contains(headerTextLower, "wins") && !strings.Contains(headerTextLower, "by")) ||
			(headerCell.Find("b").Length() > 0 && strings.Contains(headerCell.Find("b").Text(), "Wins")) {
			inWinsSection = true
			inLossesSection = false

			// Get total wins from the data cell
			dataCell := row.Find("td").First()
			if dataCell.Length() > 0 {
				totalWins = extractNumber(dataCell.Text())
			}
			return
		} else if (strings.Contains(headerTextLower, "losses") && !strings.Contains(headerTextLower, "by")) ||
			(headerCell.Find("b").Length() > 0 && strings.Contains(headerCell.Find("b").Text(), "Losses")) {
			inWinsSection = false
			inLossesSection = true

			// Get total losses from the data cell
			dataCell := row.Find("td").First()
			if dataCell.Length() > 0 {
				totalLosses = extractNumber(dataCell.Text())
			}
			return
		}

		// Extract win methods
		if inWinsSection {
			dataCell := row.Find("td").First()
			if dataCell.Length() == 0 {
				return
			}

			value := extractNumber(dataCell.Text())

			// Match different patterns for the methods
			if strings.Contains(headerTextLower, "knockout") || strings.Contains(headerTextLower, "ko") {
				info.KOWins = value
			} else if strings.Contains(headerTextLower, "submission") {
				info.SubWins = value
			} else if strings.Contains(headerTextLower, "decision") {
				info.DecWins = value
			}
		}

		// Extract loss methods
		if inLossesSection {
			dataCell := row.Find("td").First()
			if dataCell.Length() == 0 {
				return
			}

			value := extractNumber(dataCell.Text())

			// Match different patterns for the methods
			if strings.Contains(headerTextLower, "knockout") || strings.Contains(headerTextLower, "ko") {
				info.KOLosses = value
			} else if strings.Contains(headerTextLower, "submission") {
				info.SubLosses = value
			} else if strings.Contains(headerTextLower, "decision") {
				info.DecLosses = value
			} else if strings.Contains(headerTextLower, "disqualification") || strings.Contains(headerTextLower, "dq") {
				info.DQLosses = value
			}
		}

		// Comprehensive check for No Contests using multiple patterns
		isNoContests := false

		// Pattern 1: Direct text match (case insensitive)
		if strings.Contains(headerTextLower, "no") && strings.Contains(headerTextLower, "contest") {
			isNoContests = true
		}

		// Pattern 2: Bold tag with No Contests
		if headerCell.Find("b").Length() > 0 {
			boldText := strings.ToLower(headerCell.Find("b").Text())
			if strings.Contains(boldText, "no") && strings.Contains(boldText, "contest") {
				isNoContests = true
			}
		}

		// Pattern 3: HTML with non-breaking space: "No&nbsp;contests"
		if strings.Contains(headerHTML, "No&nbsp;contest") {
			isNoContests = true
		}

		if isNoContests {
			dataCell := row.Find("td").First()
			if dataCell.Length() > 0 {
				info.NoContests = extractNumber(dataCell.Text())
			}
		}
	})

	// Try direct extraction if the above didn't work
	if info.KOWins+info.SubWins+info.DecWins == 0 && totalWins > 0 {
		directExtractWinMethods(doc, info)
	}

	if info.KOLosses+info.SubLosses+info.DecLosses+info.DQLosses == 0 && totalLosses > 0 {
		directExtractLossMethods(doc, info)
	}

	// If we still couldn't find No Contests, make a separate direct search
	if info.NoContests == 0 {
		extractNoContests(doc, info)
	}

	// Validate that win methods add up to total wins
	if totalWins > 0 {
		methodsSum := info.KOWins + info.SubWins + info.DecWins
		if methodsSum > 0 && methodsSum != totalWins {
			// If they don't match, clear them
			info.KOWins = 0
			info.SubWins = 0
			info.DecWins = 0
		}
	}

	// Validate that loss methods add up to total losses
	if totalLosses > 0 {
		methodsSum := info.KOLosses + info.SubLosses + info.DecLosses + info.DQLosses
		if methodsSum > 0 && methodsSum != totalLosses {
			// If they don't match, clear them
			info.KOLosses = 0
			info.SubLosses = 0
			info.DecLosses = 0
			info.DQLosses = 0
		}
	}
}

// More direct extraction method for win methods
func directExtractWinMethods(doc *goquery.Document, info *FighterExtraInfo) {
	// Look specifically for text containing "By knockout"
	doc.Find("tr").Each(func(i int, row *goquery.Selection) {
		// Check if the row contains "By knockout" in the header
		headerCell := row.Find("th").First()
		if headerCell.Length() == 0 {
			return
		}

		headerText := headerCell.Text()

		// Skip if not a method row
		if !strings.Contains(strings.ToLower(headerText), "by knockout") &&
			!strings.Contains(strings.ToLower(headerText), "by submission") &&
			!strings.Contains(strings.ToLower(headerText), "by decision") {
			return
		}

		// Extract the value from the data cell
		dataCell := row.Find("td").First()
		if dataCell.Length() == 0 {
			return
		}

		value := extractNumber(dataCell.Text())

		// Determine what kind of method this is
		headerTextLower := strings.ToLower(headerText)
		if strings.Contains(headerTextLower, "knockout") {
			// Determine if we're in the wins or losses section by looking at previous rows
			isWinMethod := false
			row.PrevAll().Each(func(j int, prevRow *goquery.Selection) {
				prevHeader := prevRow.Find("th").First()
				if prevHeader.Length() > 0 && strings.Contains(strings.ToLower(prevHeader.Text()), "wins") {
					isWinMethod = true
					return
				}
			})

			if isWinMethod {
				info.KOWins = value
			}
		} else if strings.Contains(headerTextLower, "submission") {
			// Determine if we're in the wins or losses section
			isWinMethod := false
			row.PrevAll().Each(func(j int, prevRow *goquery.Selection) {
				prevHeader := prevRow.Find("th").First()
				if prevHeader.Length() > 0 && strings.Contains(strings.ToLower(prevHeader.Text()), "wins") {
					isWinMethod = true
					return
				}
			})

			if isWinMethod {
				info.SubWins = value
			}
		} else if strings.Contains(headerTextLower, "decision") {
			// Determine if we're in the wins or losses section
			isWinMethod := false
			row.PrevAll().Each(func(j int, prevRow *goquery.Selection) {
				prevHeader := prevRow.Find("th").First()
				if prevHeader.Length() > 0 && strings.Contains(strings.ToLower(prevHeader.Text()), "wins") {
					isWinMethod = true
					return
				}
			})

			if isWinMethod {
				info.DecWins = value
			}
		}
	})
}

// More direct extraction method for loss methods
func directExtractLossMethods(doc *goquery.Document, info *FighterExtraInfo) {
	// Look specifically for text containing method headers in the losses section
	doc.Find("tr").Each(func(i int, row *goquery.Selection) {
		// Check if the row contains a method in the header
		headerCell := row.Find("th").First()
		if headerCell.Length() == 0 {
			return
		}

		headerText := headerCell.Text()

		// Skip if not a method row
		if !strings.Contains(strings.ToLower(headerText), "by knockout") &&
			!strings.Contains(strings.ToLower(headerText), "by submission") &&
			!strings.Contains(strings.ToLower(headerText), "by decision") &&
			!strings.Contains(strings.ToLower(headerText), "by disqualification") {
			return
		}

		// Extract the value from the data cell
		dataCell := row.Find("td").First()
		if dataCell.Length() == 0 {
			return
		}

		value := extractNumber(dataCell.Text())

		// Determine what kind of method this is
		headerTextLower := strings.ToLower(headerText)
		if strings.Contains(headerTextLower, "knockout") {
			// Determine if we're in the losses section by looking at previous rows
			isLossMethod := false
			row.PrevAll().Each(func(j int, prevRow *goquery.Selection) {
				prevHeader := prevRow.Find("th").First()
				if prevHeader.Length() > 0 && strings.Contains(strings.ToLower(prevHeader.Text()), "losses") {
					isLossMethod = true
					return
				}
			})

			if isLossMethod {
				info.KOLosses = value
			}
		} else if strings.Contains(headerTextLower, "submission") {
			// Determine if we're in the losses section
			isLossMethod := false
			row.PrevAll().Each(func(j int, prevRow *goquery.Selection) {
				prevHeader := prevRow.Find("th").First()
				if prevHeader.Length() > 0 && strings.Contains(strings.ToLower(prevHeader.Text()), "losses") {
					isLossMethod = true
					return
				}
			})

			if isLossMethod {
				info.SubLosses = value
			}
		} else if strings.Contains(headerTextLower, "decision") {
			// Determine if we're in the losses section
			isLossMethod := false
			row.PrevAll().Each(func(j int, prevRow *goquery.Selection) {
				prevHeader := prevRow.Find("th").First()
				if prevHeader.Length() > 0 && strings.Contains(strings.ToLower(prevHeader.Text()), "losses") {
					isLossMethod = true
					return
				}
			})

			if isLossMethod {
				info.DecLosses = value
			}
		} else if strings.Contains(headerTextLower, "disqualification") {
			// Determine if we're in the losses section
			isLossMethod := false
			row.PrevAll().Each(func(j int, prevRow *goquery.Selection) {
				prevHeader := prevRow.Find("th").First()
				if prevHeader.Length() > 0 && strings.Contains(strings.ToLower(prevHeader.Text()), "losses") {
					isLossMethod = true
					return
				}
			})

			if isLossMethod {
				info.DQLosses = value
			}
		}
	})
}

// Extract number from text
func extractNumber(text string) int {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(text)
	if match != "" {
		num, err := strconv.Atoi(match)
		if err == nil {
			return num
		}
	}
	return 0
}

// Final validation against UFC record
func validateFighterInfo(info *FighterExtraInfo, ufcWins, ufcLosses int) {
	// Verify win methods against UFC wins
	if ufcWins > 0 {
		winMethodsSum := info.KOWins + info.SubWins + info.DecWins
		if winMethodsSum > 0 && winMethodsSum != ufcWins {
			// Methods don't match UFC total - clear them
			info.KOWins = 0
			info.SubWins = 0
			info.DecWins = 0
		}
	}

	// Verify loss methods against UFC losses
	if ufcLosses > 0 {
		lossMethodsSum := info.KOLosses + info.SubLosses + info.DecLosses + info.DQLosses
		if lossMethodsSum > 0 && lossMethodsSum != ufcLosses {
			// Methods don't match UFC total - clear them
			info.KOLosses = 0
			info.SubLosses = 0
			info.DecLosses = 0
			info.DQLosses = 0
		}
	}
}

// Add this function to extract age information
func extractAge(doc *goquery.Document) int {
	var age int

	// Try to extract age directly from the ForceAgeToShow span
	doc.Find(".noprint.ForceAgeToShow").Each(func(i int, ageSpan *goquery.Selection) {
		// Extract text from format like "(age 59)"
		ageText := ageSpan.Text()
		re := regexp.MustCompile(`\(age\D*(\d+)\)`)
		matches := re.FindStringSubmatch(ageText)
		if len(matches) > 1 {
			extractedAge, err := strconv.Atoi(matches[1])
			if err == nil {
				age = extractedAge
				return
			}
		}
	})

	// If age not found directly, try to calculate from birthdate
	if age == 0 {
		doc.Find(".bday").Each(func(i int, bdaySpan *goquery.Selection) {
			birthDateStr := bdaySpan.Text()
			// Parse birthdate in format "YYYY-MM-DD"
			birthDate, err := time.Parse("2006-01-02", birthDateStr)
			if err == nil {
				// Calculate age based on current time
				now := time.Now()
				age = now.Year() - birthDate.Year()

				// Adjust age if birthday hasn't occurred yet this year
				if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
					age--
				}
				return
			}
		})
	}

	return age
}

func extractNoContests(doc *goquery.Document, info *FighterExtraInfo) {
	// First approach: Look for specific row patterns
	doc.Find("table.infobox tr, table.vcard tr").Each(func(i int, row *goquery.Selection) {
		headerCell := row.Find("th").First()
		if headerCell.Length() == 0 {
			return
		}

		// Get both text and HTML content
		headerText := strings.TrimSpace(headerCell.Text())
		headerHTML, _ := headerCell.Html()
		headerTextLower := strings.ToLower(headerText)

		// Check for "No contests" with bold tag
		boldText := ""
		headerCell.Find("b").Each(func(j int, boldElem *goquery.Selection) {
			boldText += strings.TrimSpace(boldElem.Text()) + " "
		})
		boldTextLower := strings.ToLower(strings.TrimSpace(boldText))

		// Check for the specific format in the example
		if boldTextLower == "no contests" ||
			boldTextLower == "no contest" ||
			strings.Contains(boldTextLower, "no contest") {
			dataCell := row.Find("td").First()
			if dataCell.Length() > 0 {
				info.NoContests = extractNumber(dataCell.Text())
				fmt.Printf("Found No Contests (via bold): %d\n", info.NoContests)
				return
			}
		}

		// Check for the non-breaking space version "No&nbsp;contests"
		if strings.Contains(headerHTML, "<b>No&nbsp;contests</b>") ||
			strings.Contains(headerHTML, "<b>No&nbsp;contest</b>") ||
			strings.Contains(headerHTML, "No&nbsp;contests") ||
			strings.Contains(headerHTML, "No&nbsp;contest") {
			dataCell := row.Find("td").First()
			if dataCell.Length() > 0 {
				info.NoContests = extractNumber(dataCell.Text())
				fmt.Printf("Found No Contests (via HTML): %d\n", info.NoContests)
				return
			}
		}

		// Look for basic text match for "No contests" in the header text
		if headerTextLower == "no contests" ||
			headerTextLower == "no contest" ||
			strings.Contains(headerTextLower, "no contests") ||
			strings.Contains(headerTextLower, "no contest") ||
			(strings.Contains(headerTextLower, "no") &&
				strings.Contains(headerTextLower, "contest")) {
			dataCell := row.Find("td").First()
			if dataCell.Length() > 0 {
				info.NoContests = extractNumber(dataCell.Text())
				fmt.Printf("Found No Contests (via text): %d\n", info.NoContests)
				return
			}
		}
	})

	// Second approach: Look for rows after the "Draws" row
	if info.NoContests == 0 {
		var foundDrawsRow bool
		doc.Find("table.infobox tr, table.vcard tr").Each(func(i int, row *goquery.Selection) {
			headerCell := row.Find("th").First()
			if headerCell.Length() == 0 {
				return
			}

			headerText := strings.ToLower(strings.TrimSpace(headerCell.Text()))
			boldText := strings.ToLower(strings.TrimSpace(headerCell.Find("b").Text()))

			// Check if this is the Draws row
			if foundDrawsRow == false && (boldText == "draws" || headerText == "draws" ||
				strings.Contains(headerText, "draw") || strings.Contains(boldText, "draw")) {
				foundDrawsRow = true
				return
			}

			// If we've found the Draws row, check if the next row is No Contests
			if foundDrawsRow {
				// This might be the No Contests row - check all possible patterns
				headerTextLower := strings.ToLower(headerText)
				if strings.Contains(headerTextLower, "no") && strings.Contains(headerTextLower, "contest") {
					dataCell := row.Find("td").First()
					if dataCell.Length() > 0 {
						info.NoContests = extractNumber(dataCell.Text())
						fmt.Printf("Found No Contests (after Draws): %d\n", info.NoContests)
						foundDrawsRow = false // Reset to avoid checking further rows
						return
					}
				}
			}
		})
	}

	// Third approach: Scan the entire table for No Contest text
	if info.NoContests == 0 {
		doc.Find("table.infobox, table.vcard").Each(func(i int, table *goquery.Selection) {
			tableHTML, _ := table.Html()
			tableText := table.Text()

			// Only process if the table contains any mention of "no contest"
			if strings.Contains(strings.ToLower(tableHTML), "no contest") ||
				strings.Contains(strings.ToLower(tableText), "no contest") {

				// Try to find the specific row that contains No Contest
				table.Find("tr").Each(func(j int, row *goquery.Selection) {
					rowHTML, _ := row.Html()
					rowText := row.Text()

					if strings.Contains(strings.ToLower(rowHTML), "no contest") ||
						strings.Contains(strings.ToLower(rowText), "no contest") {

						// Found a row with "no contest" - extract the number from the TD
						dataCell := row.Find("td").First()
						if dataCell.Length() > 0 {
							info.NoContests = extractNumber(dataCell.Text())
							fmt.Printf("Found No Contests (via table search): %d\n", info.NoContests)
							return
						}
					}
				})
			}
		})
	}

	// Fourth approach: Check for infobox-label class specifically (used in your example)
	if info.NoContests == 0 {
		doc.Find("th.infobox-label").Each(func(i int, th *goquery.Selection) {
			thText := strings.ToLower(strings.TrimSpace(th.Text()))

			// Check if this is a No Contest header
			if strings.Contains(thText, "no") && strings.Contains(thText, "contest") {
				// Get the data cell (next td in same row)
				dataCell := th.Parent().Find("td.infobox-data").First()
				if dataCell.Length() > 0 {
					info.NoContests = extractNumber(dataCell.Text())
					fmt.Printf("Found No Contests (via infobox-label): %d\n", info.NoContests)
					return
				}
			}

			// Check for bold element within the header
			th.Find("b").Each(func(j int, b *goquery.Selection) {
				boldText := strings.ToLower(strings.TrimSpace(b.Text()))
				if boldText == "no contests" || boldText == "no contest" {
					dataCell := th.Parent().Find("td.infobox-data").First()
					if dataCell.Length() > 0 {
						info.NoContests = extractNumber(dataCell.Text())
						fmt.Printf("Found No Contests (via bold in infobox-label): %d\n", info.NoContests)
						return
					}
				}
			})
		})
	}
}
