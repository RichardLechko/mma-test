package scrapers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gocolly/colly/v2"
)

var (
	ErrScrapingFailed = errors.New("scraping failed")
	ErrParsingFailed  = errors.New("parsing failed")
	ErrInvalidData    = errors.New("invalid data")
)

type ScraperConfig struct {
	BaseURL          string
	UserAgent        string
	RequestTimeout   time.Duration
	RetryCount       int
	RetryWait       time.Duration
	RateLimit       time.Duration
	ParallelRequests int
}

func DefaultConfig() ScraperConfig {
	return ScraperConfig{
		BaseURL:          "https://www.ufc.com",
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko)",
		RequestTimeout:   30 * time.Second,
		RetryCount:      3,
		RetryWait:       5 * time.Second,
		RateLimit:       2 * time.Second,
		ParallelRequests: 2,
	}
}

type Scraper interface {
	Initialize(ctx context.Context) error
	Scrape(ctx context.Context, url string) error
	Close() error
}

type BaseScraper struct {
	config    ScraperConfig
	collector *colly.Collector
	client    *http.Client
}

func NewBaseScraper(config ScraperConfig) *BaseScraper {
	c := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.MaxDepth(2),
		colly.AllowedDomains("www.ufc.com", "ufc.com"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		RandomDelay: config.RateLimit,
		Parallelism: config.ParallelRequests,
	})

	client := &http.Client{
		Timeout: config.RequestTimeout,
	}

	return &BaseScraper{
		config:    config,
		collector: c,
		client:    client,
	}
}

func (s *BaseScraper) GetCollector() *colly.Collector {
	return s.collector
}

func (s *BaseScraper) Initialize(ctx context.Context) error {
	s.SetupCallbacks()
	return nil
}

func (s *BaseScraper) SetupCallbacks() {
	s.collector.OnError(func(r *colly.Response, err error) {
	})

	s.collector.OnResponse(func(r *colly.Response) {
	})
}

func (s *BaseScraper) Close() error {
	return nil
}