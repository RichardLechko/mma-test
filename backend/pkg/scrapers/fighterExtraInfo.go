package scrapers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
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
	// Win methods
	KOWins  int
	SubWins int
	DecWins int
}

type WikiFighterScraper struct {
	config *ScraperConfig
	client *http.Client
}

func NewWikiFighterScraper(config *ScraperConfig) *WikiFighterScraper {
	return &WikiFighterScraper{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *WikiFighterScraper) ScrapeExtraInfo(fighterName, wikiURL, ufcURL string, ufcWins, ufcLosses int) (*FighterExtraInfo, error) {
	var finalResp *http.Response
	var finalErr error

	// Prepare alternative URLs to try
	cleanName := strings.ReplaceAll(fighterName, " ", "_")
	cleanName = strings.ReplaceAll(cleanName, ".", "") // Remove periods
	cleanName = strings.ReplaceAll(cleanName, "'", "") // Remove apostrophes

	// Collection of URLs to try in order
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

	// Try each URL in sequence until one works
	for _, url := range allURLs {
		if url == "" {
			continue // Skip empty URLs
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", s.config.UserAgent)

		resp, err := s.client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		// Successfully got a page, now check if it's about our fighter
		doc, err := goquery.NewDocumentFromReader(resp.Body)
		resp.Body.Close() // Close the body as we've read it

		if err != nil {
			continue
		}

		// CRITICAL: Check if this is a disambiguation page
		// These pages list multiple people with the same name
		isDisambiguation := false

		// Method 1: Check for the "disambigbox" template
		if doc.Find(".disambigbox").Length() > 0 {
			isDisambiguation = true
		}

		// Method 2: Check for "may refer to" text in first paragraph
		firstPara := doc.Find(".mw-parser-output > p").First().Text()
		if strings.Contains(strings.ToLower(firstPara), "may refer to") ||
			strings.Contains(strings.ToLower(firstPara), "commonly refers to") ||
			strings.Contains(strings.ToLower(firstPara), "disambiguation") {
			isDisambiguation = true
		}

		// Method 3: Check for dmbox class
		if doc.Find(".dmbox").Length() > 0 {
			isDisambiguation = true
		}

		// Skip disambiguation pages
		if isDisambiguation {
			continue
		}

		// Extract some text to verify it's an MMA fighter page
		// We'll build up evidence
		evidenceScore := 0
		pageTitle := doc.Find("h1#firstHeading").Text()

		// Check page title - does it contain the fighter's name?
		if strings.Contains(strings.ToLower(pageTitle), strings.ToLower(fighterName)) {
			evidenceScore += 2
		}

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

		doc.Find(".mw-normal-catlinks ul li a").Each(func(i int, s *goquery.Selection) {
			category := strings.ToLower(s.Text())
			if strings.Contains(category, "mixed martial artists") ||
				strings.Contains(category, "ufc") ||
				strings.Contains(category, "ultimate fighting championship") {
				evidenceScore += 3
			}
		})

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

		if strings.Contains(lowerFullText, "featherweight") ||
			strings.Contains(lowerFullText, "lightweight") ||
			strings.Contains(lowerFullText, "welterweight") ||
			strings.Contains(lowerFullText, "middleweight") ||
			strings.Contains(lowerFullText, "heavyweight") ||
			strings.Contains(lowerFullText, "bantamweight") ||
			strings.Contains(lowerFullText, "flyweight") {
			evidenceScore += 1
		}

		if strings.Contains(lowerFullText, "professional record") ||
			strings.Contains(lowerFullText, "fight record") ||
			strings.Contains(lowerFullText, "mma record") ||
			(strings.Contains(lowerFullText, "wins") && (strings.Contains(lowerFullText, "losses") || strings.Contains(lowerFullText, "defeats"))) {
			evidenceScore += 3
		}

		doc.Find(".mw-parser-output > p").Each(func(i int, s *goquery.Selection) {
			if i > 3 {
				return
			}

			text := strings.ToLower(s.Text())
			if strings.Contains(text, "ufc") ||
				strings.Contains(text, "ultimate fighting championship") ||
				strings.Contains(text, "mixed martial") ||
				strings.Contains(text, "mma") ||
				strings.Contains(text, "fighter") ||
				strings.Contains(text, "bout") ||
				strings.Contains(text, "octagon") {
				evidenceScore += 1
			}
		})

		if evidenceScore >= 2 {
			req, _ = http.NewRequest("GET", url, nil)
			req.Header.Set("User-Agent", s.config.UserAgent)

			finalResp, err = s.client.Do(req)
			if err == nil {
				break
			}
		}
	}

	if finalResp == nil {
		return nil, fmt.Errorf("failed to access Wikipedia page for %s: %v", fighterName, finalErr)
	}

	defer finalResp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(finalResp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	info := &FighterExtraInfo{}
	info.FightingOutOf = extractFightingOutOf(doc)
	extractRecordData(doc, info)
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
