package cron

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

type JobConfig struct {
	Enabled   bool          `json:"enabled"`
	Schedule  string        `json:"schedule"`
	Timeout   time.Duration `json:"timeout"`
	RetryAttempts int      `json:"retryAttempts"`
}

type Config struct {
	Rankings JobConfig `json:"rankings"`   
	Scraping JobConfig `json:"scraping"`  
	Cleanup  JobConfig `json:"cleanup"`    
	Archive  JobConfig `json:"archive"` 
	Metrics  JobConfig `json:"metrics"`   
}

func DefaultConfig() *Config {
	return &Config{
		Rankings: JobConfig{
			Enabled:       true,
			Schedule:      "@weekly",   
			Timeout:       1 * time.Hour,
			RetryAttempts: 3,
		},
		Scraping: JobConfig{
			Enabled:       true,
			Schedule:      "@daily",     
			Timeout:       2 * time.Hour,
			RetryAttempts: 5,
		},
		Cleanup: JobConfig{
			Enabled:       true,
			Schedule:      "@daily",    
			Timeout:       30 * time.Minute,
			RetryAttempts: 2,
		},
		Archive: JobConfig{
			Enabled:       true,
			Schedule:      "@monthly",  
			Timeout:       4 * time.Hour,
			RetryAttempts: 3,
		},
		Metrics: JobConfig{
			Enabled:       true,
			Schedule:      "@hourly",   
			Timeout:       15 * time.Minute,
			RetryAttempts: 2,
		},
	}
}

func ValidateSchedule(schedule string) error {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule expression: %w", err)
	}
	return nil
}

func (c *Config) Validate() error {
	jobs := map[string]JobConfig{
		"rankings": c.Rankings,
		"scraping": c.Scraping,
		"cleanup":  c.Cleanup,
		"archive":  c.Archive,
		"metrics":  c.Metrics,
	}

	for name, job := range jobs {
		if job.Enabled {
			if err := ValidateSchedule(job.Schedule); err != nil {
				return fmt.Errorf("invalid schedule for %s job: %w", name, err)
			}

			if job.Timeout <= 0 {
				return fmt.Errorf("%s job timeout must be positive", name)
			}

			if job.RetryAttempts < 0 {
				return fmt.Errorf("%s job retry attempts must be non-negative", name)
			}
		}
	}

	return nil
}

func LoadFromJSON(data []byte) (*Config, error) {
	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func (c *Config) GetSchedule(jobName string) string {
	switch jobName {
	case "rankings":
		return c.Rankings.Schedule
	case "scraping":
		return c.Scraping.Schedule
	case "cleanup":
		return c.Cleanup.Schedule
	case "archive":
		return c.Archive.Schedule
	case "metrics":
		return c.Metrics.Schedule
	default:
		return ""
	}
}

func (c *Config) IsEnabled(jobName string) bool {
	switch jobName {
	case "rankings":
		return c.Rankings.Enabled
	case "scraping":
		return c.Scraping.Enabled
	case "cleanup":
		return c.Cleanup.Enabled
	case "archive":
		return c.Archive.Enabled
	case "metrics":
		return c.Metrics.Enabled
	default:
		return false
	}
}

func (c *Config) GetTimeout(jobName string) time.Duration {
	switch jobName {
	case "rankings":
		return c.Rankings.Timeout
	case "scraping":
		return c.Scraping.Timeout
	case "cleanup":
		return c.Cleanup.Timeout
	case "archive":
		return c.Archive.Timeout
	case "metrics":
		return c.Metrics.Timeout
	default:
		return 0
	}
}

func (c *Config) GetRetryAttempts(jobName string) int {
	switch jobName {
	case "rankings":
		return c.Rankings.RetryAttempts
	case "scraping":
		return c.Scraping.RetryAttempts
	case "cleanup":
		return c.Cleanup.RetryAttempts
	case "archive":
		return c.Archive.RetryAttempts
	case "metrics":
		return c.Metrics.RetryAttempts
	default:
		return 0
	}
}