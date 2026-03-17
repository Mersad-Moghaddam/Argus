package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config contains all runtime settings for Argus services.
type Config struct {
	HTTPAddr          string
	MySQLDSN          string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	SchedulerInterval time.Duration
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          envOrDefault("HTTP_ADDR", ":8080"),
		MySQLDSN:          envOrDefault("MYSQL_DSN", "argus:argus@tcp(localhost:3306)/argus?parseTime=true"),
		RedisAddr:         envOrDefault("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     envOrDefault("REDIS_PASSWORD", ""),
		SchedulerInterval: 30 * time.Second,
	}

	redisDB := envOrDefault("REDIS_DB", "0")
	dbIndex, err := strconv.Atoi(redisDB)
	if err != nil {
		return Config{}, fmt.Errorf("parse REDIS_DB: %w", err)
	}
	cfg.RedisDB = dbIndex

	intervalRaw := os.Getenv("SCHEDULER_INTERVAL")
	if intervalRaw == "" {
		return cfg, nil
	}

	interval, err := time.ParseDuration(intervalRaw)
	if err != nil {
		return Config{}, fmt.Errorf("parse SCHEDULER_INTERVAL: %w", err)
	}
	if interval < 5*time.Second {
		return Config{}, fmt.Errorf("SCHEDULER_INTERVAL must be at least 5s")
	}

	cfg.SchedulerInterval = interval
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
