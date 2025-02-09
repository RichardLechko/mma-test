package config

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "sync"
)

var (
    config *Config
    once   sync.Once
)

type Config struct {
    Server   ServerConfig   `json:"server"`
    Database DatabaseConfig `json:"database"`
    JWT      JWTConfig     `json:"jwt"`
    Backup   BackupConfig  `json:"backup"`
}

type ServerConfig struct {
    Port         int    `json:"port"`
    Host         string `json:"host"`
    ReadTimeout  int    `json:"read_timeout"`
    WriteTimeout int    `json:"write_timeout"`
}

type JWTConfig struct {
    Secret           string `json:"secret"`
    ExpirationHours  int    `json:"expiration_hours"`
    RefreshTokenDays int    `json:"refresh_token_days"`
}

type BackupConfig struct {
    BackupDir     string `json:"backup_dir"`
    RetentionDays int    `json:"retention_days"`
    Timeout       int    `json:"timeout"`
}

func GetConfig() *Config {
    if config == nil {
        log.Fatal("Configuration not loaded. Call LoadConfig first")
    }
    return config
}

func LoadConfig(filepath string) error {
    file, err := os.ReadFile(filepath)
    if err != nil {
        return fmt.Errorf("error reading config file: %w", err)
    }

    config = new(Config)
    if err := json.Unmarshal(file, config); err != nil {
        return fmt.Errorf("error parsing config file: %w", err)
    }

    if config.Database.Host == "" {
        return fmt.Errorf("database host is empty after loading config")
    }

    return nil
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}