package scrapers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"mma-scheduler/internal/models"

	"github.com/gocolly/colly/v2"
)

type FightScraper struct {
	processedEvents sync.Map
	client          *http.Client
}

func NewFightScraper() *FightScraper {
	return &FightScraper{
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives:   true,
				MaxIdleConns:        5,
				IdleConnTimeout:     5 * time.Second,
				TLSHandshakeTimeout: 5 * time.Second,
			},
		},
	}
}

func (s *FightScraper) ScrapeFights(eventName string, eventDate time.Time) ([]models.Fight, []string, error) {
	eventKey := normalizeEventName(eventName)
	if _, loaded := s.processedEvents.LoadOrStore(eventKey, true); loaded {
		return nil, nil, fmt.Errorf("event already processed: %s", eventName)
	}

	c := colly.NewCollector(
		colly.AllowedDomains("www.ufc.com", "ufc.com"),
		colly.MaxDepth(1),
		colly.Async(false),
	)

	c.SetClient(s.client)

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
	})

	var fights []models.Fight
	var mu sync.Mutex
	fightOrder := 1

	c.OnHTML("div.c-listing-fight", func(e *colly.HTMLElement) {
		fight := models.Fight{}

		fighters := e.ChildTexts("div.c-listing-fight__corner-name")
		if len(fighters) >= 2 {
			fight.Fighter1 = cleanFighterName(fighters[0])
			fight.Fighter2 = cleanFighterName(fighters[1])
		}

		weightClass := e.ChildText("div.c-listing-fight__class-text")
		if weightClass == "" {
			weightClass = e.ChildText("div.c-listing-fight__class")
		}
		fight.WeightClass = cleanWeightClass(weightClass)

		isMain := e.ChildText("div.c-listing-fight__banner-title")
		fight.IsMainEvent = strings.Contains(strings.ToLower(isMain), "main event")

		fight.Order = fightOrder

		if fight.Fighter1 != "" && fight.Fighter2 != "" {
			mu.Lock()
			fights = append(fights, fight)
			fightOrder++
			mu.Unlock()
		}
	})

	attemptedURLs := []string{}
	urls := generatePossibleURLs(eventName, eventDate)

	var lastErr error
	for _, url := range urls {
		attemptedURLs = append(attemptedURLs, url)

		err := c.Visit(url)
		if err != nil {
			lastErr = err
			continue
		}

		c.Wait()

		if len(fights) > 0 {
			return fights, attemptedURLs, nil
		}
	}

	if lastErr != nil {
		return nil, attemptedURLs, fmt.Errorf("failed to scrape fights: %v", lastErr)
	}

	return nil, attemptedURLs, fmt.Errorf("no fights found for event: %s", eventName)
}

func generatePossibleURLs(eventName string, eventDate time.Time) []string {
	baseURL := "https://www.ufc.com/event/"
	normalizedName := strings.ToLower(eventName)
	var urls []string

	// Priority 1: Numbered UFC events
	if match := regexp.MustCompile(`ufc (\d+)`).FindStringSubmatch(normalizedName); len(match) > 1 {
		urls = append(urls, baseURL+"ufc-"+match[1])
		return urls
	}

	// Priority 2: Handle special events
	if strings.Contains(normalizedName, "noche") {
		urls = append(urls, baseURL+"noche-ufc")
		if eventDate.Year() > 0 {
			urls = append(urls, fmt.Sprintf("%snoche-ufc-%d", baseURL, eventDate.Year()))
		}
	}

	// Priority 3: Fight Night with fighter names
	if strings.Contains(normalizedName, "fight night") || strings.Contains(normalizedName, "vs") {
		name := strings.TrimPrefix(normalizedName, "ufc fight night:")
		name = strings.TrimPrefix(name, "ufc fight night")
		name = strings.TrimPrefix(name, "ufc")
		name = strings.TrimSpace(name)

		for _, sep := range []string{" vs. ", " vs ", ": ", " - "} {
			if parts := strings.Split(name, sep); len(parts) == 2 {
				fighter1 := cleanNameForURL(parts[0])
				fighter2 := cleanNameForURL(parts[1])
				if fighter1 != "" && fighter2 != "" {
					urls = append(urls,
						fmt.Sprintf("%sufc-fight-night-%s-vs-%s", baseURL, fighter1, fighter2),
						fmt.Sprintf("%sufc-fight-night-%s-vs-%s", baseURL, fighter2, fighter1))
				}
			}
		}
	}

	// Priority 4: Date-based URLs
	if strings.Contains(normalizedName, "fight night") {
		monthStr := strings.ToLower(eventDate.Month().String())
		monthAbbr := monthStr[:3]

		urls = append(urls,
			fmt.Sprintf("%sufc-fight-night-%s-%d-%d", baseURL, monthStr, eventDate.Day(), eventDate.Year()),
			fmt.Sprintf("%sufc-fight-night-%s-%d-%d", baseURL, monthAbbr, eventDate.Day(), eventDate.Year()),
			fmt.Sprintf("%sufc-fight-night-%s-%02d-%d", baseURL, monthStr, eventDate.Day(), eventDate.Year()))
	}

	// Priority 5: Clean event name as fallback
	name := cleanEventNameForURL(eventName)
	if name != "" {
		if strings.Contains(normalizedName, "fight night") {
			urls = append(urls, baseURL+"ufc-fight-night-"+name)
		} else {
			urls = append(urls, baseURL+"ufc-"+name)
		}
	}

	return removeDuplicateURLs(urls)
}

func cleanNameForURL(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = regexp.MustCompile(`[^a-z0-9 ]`).ReplaceAllString(name, "")
	name = strings.Join(strings.Fields(name), "-")
	return name
}

func cleanEventNameForURL(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, ": ", "-")
	name = strings.ReplaceAll(name, " - ", "-")
	name = strings.ReplaceAll(name, " vs. ", "-vs-")
	name = strings.ReplaceAll(name, " vs ", "-vs-")
	name = strings.ReplaceAll(name, " ", "-")
	name = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(name, "")

	// Clean up multiple dashes
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	return strings.Trim(name, "-")
}

func normalizeEventName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(name, "")
	return name
}

func removeDuplicateURLs(urls []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(urls))

	for _, url := range urls {
		url = strings.TrimRight(url, "-")
		if !seen[url] {
			seen[url] = true
			result = append(result, url)
		}
	}

	return result
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
