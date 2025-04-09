package scrapers

import (
	"net/http"
	"time"
)

// ScraperConfig contains configuration for scrapers
type ScraperConfig struct {
	UserAgent    string
	Timeout      time.Duration
	MaxRetries   int
	RetryDelay   time.Duration
	ProxyURL     string
}

// DefaultConfig creates a default scraper configuration
func DefaultConfig() ScraperConfig {
	return ScraperConfig{
		UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
		ProxyURL:     "",
	}
}

// BaseScraper provides common functionality for scrapers
type BaseScraper struct {
	config ScraperConfig
	client *http.Client
}

// NewBaseScraper creates a new base scraper
func NewBaseScraper(config ScraperConfig) *BaseScraper {
	client := &http.Client{
		Timeout: config.Timeout,
	}
	
	// Configure proxy if provided
	if config.ProxyURL != "" {
		// Proxy configuration would go here
	}
	
	return &BaseScraper{
		config: config,
		client: client,
	}
}