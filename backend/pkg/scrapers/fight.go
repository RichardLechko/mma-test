package scrapers

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"mma-scheduler/internal/models"
	"net/http"
	"strings"
	"time"
)

type FightScraper struct {
	collector *colly.Collector
}

func NewFightScraper() *FightScraper {
	c := colly.NewCollector(
		colly.AllowedDomains("www.ufc.com", "ufc.com"),
		colly.MaxDepth(1),
		colly.Async(false),
	)

	c.SetRequestTimeout(10 * time.Second)

	c.WithTransport(&http.Transport{
		DisableKeepAlives:   true,
		MaxIdleConns:        5,
		IdleConnTimeout:     5 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
	})

	return &FightScraper{
		collector: c,
	}
}

func (s *FightScraper) validateURL(url string) bool {
	resp, err := http.Head(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *FightScraper) ScrapeFights(eventName string, eventDate time.Time) ([]models.Fight, []string, error) {
    var fights []models.Fight
    c := s.collector.Clone()

    eventURL := formatEventURL(eventName, eventDate)
    baseURL := "https://www.ufc.com/event/"

    // Generate potential URL variations
    urls := []string{
        baseURL + eventURL,
    }

	// For Fight Nights, try alternative formats
    if strings.Contains(strings.ToLower(eventName), "fight night") {
        name := strings.ToLower(eventName)
        if strings.Contains(name, "vs.") {
            parts := strings.Split(name, ":")
            if len(parts) > 1 {
                fighters := strings.Split(parts[1], "vs.")
                if len(fighters) == 2 {
                    fighter1 := strings.TrimSpace(strings.ReplaceAll(fighters[0], ".", ""))
                    fighter2 := strings.TrimSpace(strings.ReplaceAll(fighters[1], ".", ""))
                    altURL := fmt.Sprintf("%sufc-fight-night-%s-%s",
                        baseURL,
                        strings.ToLower(strings.ReplaceAll(fighter1, " ", "-")),
                        strings.ToLower(strings.ReplaceAll(fighter2, " ", "-")))
                    urls = append(urls, altURL)
                }
            }
        }
    }

    // Add new alternative URLs for old event formats
    if strings.Contains(strings.ToLower(eventName), "on fox") {
        monthName := strings.ToLower(eventDate.Month().String())
        year := eventDate.Year()
        urls = append(urls, fmt.Sprintf("%sufc-fight-night-%s-%d", baseURL, monthName, year))
    }

    if strings.Contains(strings.ToLower(eventName), "on fuel") {
        monthName := strings.ToLower(eventDate.Month().String())
        year := eventDate.Year()
        urls = append(urls, fmt.Sprintf("%sufc-fight-night-%s-%d", baseURL, monthName, year))
    }

	// Try each URL until we find one that works
	var lastErr error
	for _, url := range urls {
		if !s.validateURL(url) {
			continue
		}

		err := c.Visit(url)
		if err != nil {
			lastErr = err
			continue
		}

		var fightOrder int = 1
		c.OnHTML("div.c-listing-fight", func(e *colly.HTMLElement) {
			fight := models.Fight{}

			fighters := e.ChildTexts("div.c-listing-fight__corner-name")
			if len(fighters) >= 2 {
				fight.Fighter1 = cleanFighterName(fighters[0])
				fight.Fighter2 = cleanFighterName(fighters[1])
			}

			var weightClass string
			e.ForEach("div.c-listing-fight__class-text", func(i int, s *colly.HTMLElement) {
				if i == 0 {
					weightClass = s.Text
				}
			})
			if weightClass == "" {
				e.ForEach("div.c-listing-fight__class", func(i int, s *colly.HTMLElement) {
					if i == 0 {
						weightClass = s.Text
					}
				})
			}
			fight.WeightClass = cleanWeightClass(weightClass)

			isMain := e.ChildText("div.c-listing-fight__banner-title")
			fight.IsMainEvent = strings.Contains(strings.ToLower(isMain), "main event")

			fight.Order = fightOrder
			fightOrder++

			if fight.Fighter1 != "" && fight.Fighter2 != "" {
				fights = append(fights, fight)
			}
		})

		c.Wait()

		if len(fights) > 0 {
			return fights, urls, nil
		}
	}

	// If we got here, none of the URLs worked
	if lastErr != nil {
		return nil, urls, fmt.Errorf("failed to scrape fights from any URL format: %v", lastErr)
	}

	return fights, urls, nil
}

func formatEventURL(eventName string, eventDate time.Time) string {
	name := strings.ToLower(eventName)

	// Normalize dashes and special characters first
	name = strings.ReplaceAll(name, "–", "-") // Convert en dash to regular dash
	name = strings.ReplaceAll(name, "—", "-") // Convert em dash to regular dash

	// Special case for Riyadh Season events
	if strings.Contains(name, "riyadh season") {
		name = strings.ReplaceAll(name, "ufc ", "")
		name = strings.ReplaceAll(name, " - ", "-")
		name = strings.ReplaceAll(name, ": ", "-")
		return strings.ToLower(name)
	}

	// Handle UFC numbered events
	if strings.Contains(name, "ufc ") {
		numStr := strings.TrimPrefix(name, "ufc ")
		if idx := strings.Index(numStr, ":"); idx != -1 {
			numStr = numStr[:idx]
		}
		numStr = strings.TrimSpace(numStr)
		if _, err := fmt.Sscanf(numStr, "%d", new(int)); err == nil {
			return fmt.Sprintf("ufc-%s", numStr)
		}
	}

	// Handle Fight Nights with proper date format
	if strings.Contains(name, "fight night") {
		loc, _ := time.LoadLocation("UTC")
		eventDate = eventDate.In(loc)

		// Extract fighter names if present
		var fighter1, fighter2 string
		if strings.Contains(name, "vs.") || strings.Contains(name, "vs") {
			parts := strings.Split(name, ":")
			if len(parts) > 1 {
				fighterPart := parts[1]
				// Handle both "vs." and "vs" cases
				fighters := strings.Split(fighterPart, "vs.")
				if len(fighters) != 2 {
					fighters = strings.Split(fighterPart, "vs")
				}
				if len(fighters) == 2 {
					fighter1 = strings.TrimSpace(strings.ReplaceAll(fighters[0], ".", ""))
					fighter2 = strings.TrimSpace(strings.ReplaceAll(fighters[1], ".", ""))

					// Clean fighter names
					fighter1 = strings.ToLower(strings.TrimSpace(fighter1))
					fighter2 = strings.ToLower(strings.TrimSpace(fighter2))

					// Remove any remaining dots and special characters
					fighter1 = strings.ReplaceAll(fighter1, ".", "")
					fighter2 = strings.ReplaceAll(fighter2, ".", "")
				}
			}
		}

		// For events before 2021, try the fighter names format first
		if eventDate.Year() < 2021 && fighter1 != "" && fighter2 != "" {
			url := fmt.Sprintf("ufc-fight-night-%s-%s",
				strings.ToLower(fighter1),
				strings.ToLower(fighter2))
			url = strings.ReplaceAll(url, " ", "-")
			return url
		}

		// For older events, include city if available
		if strings.Contains(name, "raleigh") {
			return fmt.Sprintf("ufc-fight-night-raleigh-%s-%d-%s-%s",
				strings.ToLower(eventDate.Month().String()),
				eventDate.Day(),
				fighter1,
				fighter2)
		}

		// Default format with date
		monthName := strings.ToLower(eventDate.Month().String())
		day := eventDate.Day()
		year := eventDate.Year()

		// Generate all possible URL formats
		urls := []string{
			// Date-based formats
			fmt.Sprintf("ufc-fight-night-%s-%02d-%d", monthName, day, year),
			fmt.Sprintf("ufc-fight-night-%s-%d-%d", monthName, day, year),

			// Month and day format without year
			fmt.Sprintf("ufc-fight-night-%s-%02d", monthName, day),
			fmt.Sprintf("ufc-fight-night-%s-%d", monthName, day),

			// Simple date format
			fmt.Sprintf("ufc-fight-night-%s-%d", monthName, day),

			// Fighter-based format without date
			fmt.Sprintf("ufc-fight-night-%s-vs-%s", fighter1, fighter2),

			// Location-based format if exists
			fmt.Sprintf("ufc-fight-night-%s-%s-%d", monthName, fighter1, year),

			// Date with fighters format
			fmt.Sprintf("ufc-fight-night-%s-%02d-%s-vs-%s", monthName, day, fighter1, fighter2),
		}

		// If we have fighter names, add that format too
		if fighter1 != "" && fighter2 != "" {
			urls = append(urls, fmt.Sprintf("ufc-fight-night-%s-%d-%s-%s",
				monthName, day, strings.ToLower(fighter1), strings.ToLower(fighter2)))
		}

		// Return the first URL for now, but we could implement URL checking here
		return urls[0]
	}

	// Fallback: clean up the name
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, ":", "")
	name = strings.ReplaceAll(name, "'", "")

	// Remove double dashes
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	url := strings.Trim(name, "-")
	fmt.Printf("Event: %s -> URL: %s\n", eventName, url)
	return url
}

func cleanFighterName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\n", " ")
	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}
	return name
}

func cleanWeightClass(weightClass string) string {
	weightClass = strings.TrimSpace(weightClass)
	weightClass = strings.ToUpper(weightClass)

	weightClass = strings.ReplaceAll(weightClass, "BOUT", "")
	weightClass = strings.ReplaceAll(weightClass, "TITLE", "")
	weightClass = strings.ReplaceAll(weightClass, "POUND", "")
	weightClass = strings.ReplaceAll(weightClass, "LBS", "")

	if strings.Contains(weightClass, "WOMEN'S") || strings.Contains(weightClass, "WOMENS") {
		weightClass = strings.ReplaceAll(weightClass, "WOMEN'S", "W")
		weightClass = strings.ReplaceAll(weightClass, "WOMENS", "W")
	}

	weightClass = strings.ReplaceAll(weightClass, "WSTRAW", "WSTRAWWEIGHT")
	weightClass = strings.ReplaceAll(weightClass, "WFLY", "WFLYWEIGHT")
	weightClass = strings.ReplaceAll(weightClass, "WBANTAM", "WBANTAMWEIGHT")
	weightClass = strings.ReplaceAll(weightClass, "WFEATHER", "WFEATHERWEIGHT")

	for strings.Contains(weightClass, "  ") {
		weightClass = strings.ReplaceAll(weightClass, "  ", " ")
	}

	return strings.TrimSpace(weightClass)
}
