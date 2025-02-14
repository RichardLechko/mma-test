package scrapers

import (
	"encoding/json"
	"fmt"
	"io"
	"mma-scheduler/internal/models"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type FighterScraper struct {
	*BaseScraper
}

func NewFighterScraper(config ScraperConfig) *FighterScraper {
	return &FighterScraper{
		BaseScraper: NewBaseScraper(config),
	}
}

func (s *FighterScraper) ScrapeFighters() ([]*models.Fighter, error) {
	var allFighters []*models.Fighter
	var mutex sync.Mutex
	var errorLog []string
	page := 0
	maxPages := 300
	workerCount := 5

	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	resp, err := client.Get("https://www.ufc.com/athletes/all")
	if err != nil {
		return nil, fmt.Errorf("error fetching first page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing first page HTML: %v", err)
	}

	viewDomID := ""
	doc.Find("div.view").Each(func(i int, s *goquery.Selection) {
		if id, exists := s.Attr("data-view-id"); exists {
			if strings.Contains(id, "all_athletes") {
				if domID, exists := s.Attr("data-view-dom-id"); exists {
					viewDomID = domID
				}
			}
		}
	})

	if viewDomID == "" {
		doc.Find("div[class*='js-view-dom-id-']").Each(func(i int, s *goquery.Selection) {
			if class, exists := s.Attr("class"); exists {
				re := regexp.MustCompile(`js-view-dom-id-([a-zA-Z0-9]+)`)
				if matches := re.FindStringSubmatch(class); len(matches) > 1 {
					viewDomID = matches[1]
				}
			}
		})
	}

	if viewDomID == "" {
		return nil, fmt.Errorf("could not find view DOM ID")
	}

	fmt.Printf("Using view DOM ID: %s\n", viewDomID)

	jobs := make(chan string, 100)
	results := make(chan *models.Fighter, 100)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range jobs {
				maxRetries := 3
				var fighter *models.Fighter

				for attempt := 1; attempt <= maxRetries; attempt++ {
					req, reqErr := http.NewRequest("GET", url, nil)
					if reqErr != nil {
						continue
					}

					req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/123.0")

					resp, err := client.Do(req)
					if err != nil {
						if attempt < maxRetries {
							time.Sleep(time.Second * time.Duration(attempt))
							continue
						}
						mutex.Lock()
						errorLog = append(errorLog, fmt.Sprintf("Error scraping %s (attempt %d): %v", url, attempt, err))
						mutex.Unlock()
						break
					}

					doc, err := goquery.NewDocumentFromReader(resp.Body)
					resp.Body.Close()
					if err != nil {
						if attempt < maxRetries {
							time.Sleep(time.Second * time.Duration(attempt))
							continue
						}
						mutex.Lock()
						errorLog = append(errorLog, fmt.Sprintf("Error parsing %s (attempt %d): %v", url, attempt, err))
						mutex.Unlock()
						break
					}

					fighter = &models.Fighter{}
					fighter.UFCID = strings.TrimPrefix(url, "https://www.ufc.com/athlete/")
					fighter.Name = strings.TrimSpace(doc.Find("h1.hero-profile__name").Text())
					fighter.Nickname = strings.TrimSpace(doc.Find("p.hero-profile__nickname").Text())
					fighter.Nickname = strings.Trim(fighter.Nickname, "\"")
					fighter.WeightClass = strings.TrimSpace(strings.Replace(
						doc.Find(".hero-profile__division-title").Text(),
						" Division",
						"",
						-1,
					))

					recordText := doc.Find(".hero-profile__division-body").Text()
					if matches := regexp.MustCompile(`(\d+)-(\d+)-(\d+)`).FindStringSubmatch(recordText); len(matches) == 4 {
						fighter.Record.Wins, _ = strconv.Atoi(matches[1])
						fighter.Record.Losses, _ = strconv.Atoi(matches[2])
						fighter.Record.Draws, _ = strconv.Atoi(matches[3])
					}

					doc.Find(".hero-profile__tags p").Each(func(i int, s *goquery.Selection) {
						text := strings.TrimSpace(s.Text())
						if strings.Contains(text, "#") {
							fighter.Rank = strings.TrimSpace(strings.Split(text, " ")[0])
						}
						if text == "Active" {
							fighter.Status = "Active"
						}
					})

					doc.Find(".hero-profile__stats .hero-profile__stat").Each(func(i int, s *goquery.Selection) {
						number := strings.TrimSpace(s.Find(".hero-profile__stat-numb").Text())
						statText := strings.TrimSpace(s.Find(".hero-profile__stat-text").Text())

						num, _ := strconv.Atoi(number)
						switch statText {
						case "Wins by Knockout":
							fighter.Record.KOWins = num
						case "Wins by Submission":
							fighter.Record.SubWins = num
						case "First Round Finishes":
							fighter.FirstRound = num
						}
					})

					results <- fighter
					break
				}
			}
		}()
	}

	totalProcessed := 0
	go func() {
		for fighter := range results {
			mutex.Lock()
			allFighters = append(allFighters, fighter)
			totalProcessed++
			mutex.Unlock()
		}
	}()

	for page < maxPages {
		var currentDoc *goquery.Document

		if page == 0 {
			currentDoc = doc
		} else {
			ajaxURL := fmt.Sprintf(
				"https://www.ufc.com/views/ajax?view_name=all_athletes&view_display_id=page&view_args=&view_path=%%2Fathletes%%2Fall&view_base_path=&view_dom_id=%s&pager_element=0&page=%d&_=%d",
				viewDomID,
				page,
				time.Now().UnixNano()/int64(time.Millisecond),
			)

			req, err := http.NewRequest("GET", ajaxURL, nil)
			if err != nil {
				return nil, fmt.Errorf("error creating request for page %d: %v", page, err)
			}

			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/123.0")
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
			req.Header.Set("Referer", "https://www.ufc.com/athletes/all")

			ajaxResp, err := client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("error fetching page %d: %v", page, err)
			}
			defer ajaxResp.Body.Close()

			body, err := io.ReadAll(ajaxResp.Body)
			if err != nil {
				return nil, fmt.Errorf("error reading response body: %v", err)
			}

			var ajaxData []map[string]interface{}
			if err := json.Unmarshal(body, &ajaxData); err != nil {
				return nil, fmt.Errorf("error parsing JSON page %d: %v", page, err)
			}

			var htmlContent string
			for _, item := range ajaxData {
				if method, ok := item["method"].(string); ok && method == "infiniteScrollInsertView" {
					if data, ok := item["data"].(string); ok {
						htmlContent = data
						break
					}
				}
			}

			if htmlContent == "" {
				break
			}

			currentDoc, err = goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
			if err != nil {
				return nil, fmt.Errorf("error parsing AJAX HTML: %v", err)
			}
		}

		var pageURLs []string
		currentDoc.Find("a[href*='/athlete/']").Each(func(i int, s *goquery.Selection) {
			if href, exists := s.Attr("href"); exists {
				if strings.Contains(href, "/athlete/") && !strings.Contains(href, "#") {
					fullURL := "https://www.ufc.com" + href
					if !strings.Contains(strings.Join(pageURLs, ""), fullURL) {
						pageURLs = append(pageURLs, fullURL)
					}
				}
			}
		})

		for _, url := range pageURLs {
			jobs <- url
		}

		if len(pageURLs) == 0 {
			fmt.Printf("No fighters found on page %d, stopping\n", page)
			break
		}

		for _, url := range pageURLs {
			jobs <- url
		}

		time.Sleep(1 * time.Second)
		page++

		if page >= maxPages {
			fmt.Printf("Reached maximum page limit of %d\n", maxPages)
			break
		}
	}

	close(jobs)
	wg.Wait()
	close(results)

	fmt.Printf("Total fighters found: %d\n", len(allFighters))
	if len(errorLog) > 0 {
		fmt.Printf("\nErrors encountered (%d):\n", len(errorLog))
		for _, err := range errorLog {
			fmt.Println(err)
		}
	}

	return allFighters, nil
}

func (s *FighterScraper) ScrapeFighter(url string) (*models.Fighter, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/123.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching fighter: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	fighter := &models.Fighter{}

	if parts := strings.Split(url, "/"); len(parts) > 0 {
		fighter.UFCID = parts[len(parts)-1]
	}

	fighter.Name = strings.TrimSpace(doc.Find("h1.hero-profile__name").Text())

	fighter.Nickname = strings.TrimSpace(doc.Find("p.hero-profile__nickname").Text())
	fighter.Nickname = strings.Trim(fighter.Nickname, "\"")

	fighter.WeightClass = strings.TrimSpace(strings.Replace(
		doc.Find(".hero-profile__division-title").Text(),
		" Division",
		"",
		-1,
	))

	recordText := doc.Find(".hero-profile__division-body").Text()
	if matches := regexp.MustCompile(`(\d+)-(\d+)-(\d+)`).FindStringSubmatch(recordText); len(matches) == 4 {
		fighter.Record.Wins, _ = strconv.Atoi(matches[1])
		fighter.Record.Losses, _ = strconv.Atoi(matches[2])
		fighter.Record.Draws, _ = strconv.Atoi(matches[3])
	}

	doc.Find(".hero-profile__tags p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "#") {
			fighter.Rank = strings.TrimSpace(strings.Split(text, " ")[0])
		}
		if text == "Active" {
			fighter.Status = "Active"
		}
	})

	doc.Find(".hero-profile__stats .hero-profile__stat").Each(func(i int, s *goquery.Selection) {
		number := strings.TrimSpace(s.Find(".hero-profile__stat-numb").Text())
		statText := strings.TrimSpace(s.Find(".hero-profile__stat-text").Text())

		num, _ := strconv.Atoi(number)
		switch statText {
		case "Wins by Knockout":
			fighter.Record.KOWins = num
		case "Wins by Submission":
			fighter.Record.SubWins = num
		case "First Round Finishes":
			fighter.FirstRound = num
		}
	})

	return fighter, nil
}
