package scrapers

import (
	"context"
	"errors"
	"net"
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
	UserAgent          string
	Timeout            time.Duration 
	RetryCount         int
	RetryWaitTime      time.Duration
	MaxIdleConns       int
	MaxIdleConnsPerHost int
	MaxConnsPerHost    int
	IdleConnTimeout    time.Duration
}

func DefaultConfig() ScraperConfig {
    return ScraperConfig{
        UserAgent:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko)",
        Timeout:            30 * time.Second,
        RetryCount:         3,
        RetryWaitTime:      5 * time.Second,
        MaxIdleConns:       100,
        MaxIdleConnsPerHost: 20,
        MaxConnsPerHost:     20,
        IdleConnTimeout:     90 * time.Second,
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
	// Create a custom transport with optimized connection pooling
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}

	// Create HTTP client with the optimized transport
	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	return &BaseScraper{
		config: config,
		client: client,
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