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
	cleanName = strings.ReplaceAll(cleanName, "'", "") // Remove apostrophes

	// Collection of URLs to try
	allURLs := []string{
		// Original URL passed in (if it exists)
		wikiURL,
		// Generic name format (most common for fighters according to your observation)
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s", cleanName),
		// More specific formats as fallbacks
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(fighter)", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(mixed_martial_artist)", cleanName),
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(martial_artist)", cleanName),
		// For TUF contestants, try this format last
		fmt.Sprintf("https://en.wikipedia.org/wiki/%s_(The_Ultimate_Fighter)", cleanName),
	}

	// Filter out empty URLs
	var urls []string
	for _, url := range allURLs {
		if url != "" {
			urls = append(urls, url)
		}
	}

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	
	// Create a channel to receive successful results
	resultChan := make(chan *goquery.Document, len(urls))
	
	// Create a context with cancellation for early termination
	fetchCtx, fetchCancel := context.WithCancel(ctx)
	defer fetchCancel()

	// Launch concurrent requests to all URLs
	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			
			// Try to fetch and validate the URL
			doc, err := s.fetchURL(fetchCtx, url, fighterName)
			if err == nil && doc != nil {
				// Successfully found a valid page, send it to the result channel
				select {
				case resultChan <- doc:
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
	select {
	case doc = <-resultChan:
		// We got a valid document
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout while fetching Wikipedia page for %s", fighterName)
	}

	// If we didn't get a valid document, return an error
	if doc == nil {
		return nil, fmt.Errorf("failed to find a valid Wikipedia page for %s", fighterName)
	}

	// Use goroutines to extract different types of information in parallel
	var wgExtract sync.WaitGroup
	info := &FighterExtraInfo{}
	
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
	
	// Wait for all extraction goroutines to complete
	wgExtract.Wait()
	
	// Final validation
	validateFighterInfo(info, ufcWins, ufcLosses)

	if isEmpty(info) {
		return nil, nil
	}

	return info, nil
}

// Check if the fighter info is empty
func isEmpty(info *FighterExtraInfo) bool {
	return info.KOLosses == 0 &&
		info.SubLosses == 0 &&
		info.DecLosses == 0 &&
		info.DQLosses == 0 &&
		info.NoContests == 0 &&
		info.FightingOutOf == "" &&
		info.KOWins == 0 &&
		info.SubWins == 0 &&
		info.DecWins == 0
}

// Extract the "Fighting out of" information
func extractFightingOutOf(doc *goquery.Document) string {
	var result string

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
				// Get the raw text and clean it
				rawText := dataCell.Text()

				// Remove citations like [1], [2], etc.
				re := regexp.MustCompile(`\[\d+\]`)
				text := re.ReplaceAllString(rawText, "")

				// Replace multiple spaces with a single space
				text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

				result = strings.TrimSpace(text)
			}
		}
	})

	return result
}

// Extract record data from the infobox or record table
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

		// Special handling for No Contests with multiple patterns
		// This is a critical fix - we need to check multiple patterns including HTML
		noContestsMatches := false

		// Check text version
		if strings.Contains(headerTextLower, "no contest") {
			noContestsMatches = true
		}

		// Check HTML version for non-breaking space: "No&nbsp;contests"
		if strings.Contains(headerHTML, "No&nbsp;contest") {
			noContestsMatches = true
		}

		// Check bold tag version: <b>No contests</b>
		if strings.Contains(headerHTML, "<b>No") && strings.Contains(headerHTML, "contest") {
			noContestsMatches = true
		}

		// If any pattern matched, extract the value
		if noContestsMatches {
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

	// If we still couldn't find No Contests, make a direct search for it
	if info.NoContests == 0 {
		doc.Find("tr").Each(func(i int, row *goquery.Selection) {
			headerCell := row.Find("th").First()
			if headerCell.Length() == 0 {
				return
			}

			// Get both the raw HTML and text
			headerHTML, _ := headerCell.Html()

			// Try additional patterns for No Contests
			if strings.Contains(headerHTML, "No&nbsp;contests") ||
				strings.Contains(headerHTML, "<b>No contests</b>") ||
				strings.Contains(headerHTML, "<b>No&nbsp;contests</b>") {
				dataCell := row.Find("td").First()
				if dataCell.Length() > 0 {
					info.NoContests = extractNumber(dataCell.Text())
				}
			}
		})
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