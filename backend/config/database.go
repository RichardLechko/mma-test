package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type DatabaseConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Database     string `json:"database"`
	User         string `json:"user"`
	Password     string `json:"password"`
	ProjectID    string `json:"project_id"`
	APIKey       string `json:"api_key"`
	ProjectURL   string `json:"project_url"`
	MaxOpenConns int    `json:"max_open_conns"`
	MaxIdleConns int    `json:"max_idle_conns"`
}

func GetDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Host:         getEnvOrDefault("SUPABASE_HOST", "db.example.supabase.co"),
		Port:         5432,
		Database:     getEnvOrDefault("SUPABASE_DB_NAME", "postgres"),
		User:         getEnvOrDefault("SUPABASE_USER", "postgres"),
		Password:     getEnvOrDefault("SUPABASE_PASSWORD", ""),
		ProjectID:    getEnvOrDefault("SUPABASE_PROJECT_ID", ""),
		APIKey:       getEnvOrDefault("SUPABASE_API_KEY", ""),
		ProjectURL:   getEnvOrDefault("SUPABASE_PROJECT_URL", ""),
		MaxOpenConns: 25,
		MaxIdleConns: 25,
	}
}

func GetDatabaseURL() string {
	config := GetDatabaseConfig()
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)
}

func CreateSampleConfig(filepath string) error {
	config := Config{
		Database: DatabaseConfig{
			Host:         "your-project.supabase.co",
			Port:         5432,
			Database:     "postgres",
			User:         "postgres",
			Password:     "your-password",
			ProjectID:    "your-project-id",
			APIKey:       "your-api-key",
			ProjectURL:   "https://your-project.supabase.co",
			MaxOpenConns: 25,
			MaxIdleConns: 25,
		},
	}

	configJSON, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("error creating sample config: %w", err)
	}

	if err := os.WriteFile(filepath, configJSON, 0644); err != nil {
		return fmt.Errorf("error writing sample config: %w", err)
	}

	return nil
}