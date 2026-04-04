package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr string
	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	SchedulerInterval   time.Duration
	WorkerConcurrency   int
	QueueCriticalWeight int
	QueueDefaultWeight  int
	DueCheckBatchSize   int
	APIKey              string

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()
	cfg := Config{HTTPAddr: envOrDefault("HTTP_ADDR", ":8080"), MySQLDSN: envOrDefault("MYSQL_DSN", "argus:argus@tcp(localhost:3306)/argus?parseTime=true"), RedisAddr: envOrDefault("REDIS_ADDR", "localhost:6379"), RedisPassword: envOrDefault("REDIS_PASSWORD", ""), SchedulerInterval: mustDuration("SCHEDULER_INTERVAL", 30*time.Second), WorkerConcurrency: mustInt("WORKER_CONCURRENCY", 10), QueueCriticalWeight: mustInt("QUEUE_CRITICAL_WEIGHT", 6), QueueDefaultWeight: mustInt("QUEUE_DEFAULT_WEIGHT", 4), DueCheckBatchSize: mustInt("DUE_CHECK_BATCH_SIZE", 200), APIKey: os.Getenv("API_KEY"), DBMaxOpenConns: mustInt("DB_MAX_OPEN_CONNS", 25), DBMaxIdleConns: mustInt("DB_MAX_IDLE_CONNS", 25), DBConnMaxLifetime: mustDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)}

	dbIndex, err := strconv.Atoi(envOrDefault("REDIS_DB", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("parse REDIS_DB: %w", err)
	}
	cfg.RedisDB = dbIndex
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func mustInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
func mustDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
