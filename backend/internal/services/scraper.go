package services

import (
	"net/http"
	"time"
)

type ScraperConfig struct {
	UserAgent  string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
	ProxyURL   string
}

func DefaultScraperConfig() ScraperConfig {
	return ScraperConfig{
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		ProxyURL:   "",
	}
}

type BaseScraper struct {
	config ScraperConfig
	client *http.Client
}

func NewBaseScraper(config ScraperConfig) *BaseScraper {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	if config.ProxyURL != "" {
	}

	return &BaseScraper{
		config: config,
		client: client,
	}
}
