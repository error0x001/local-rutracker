package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DB       DBConfig
	Server   ServerConfig
	Migrator MigratorConfig
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type MigratorConfig struct {
	FilePath      string
	BatchSize     int
	ProgressEvery int
}

func Load() *Config {
	cfg := &Config{
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "rutracker"),
			Password: getEnv("DB_PASSWORD", "rutracker"),
			DBName:   getEnv("DB_NAME", "rutracker"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			Addr:         getEnv("SERVER_ADDR", ":8080"),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
		},
		Migrator: MigratorConfig{
			FilePath:      getEnv("MIGRATOR_FILE", ""),
			BatchSize:     getEnvInt("MIGRATOR_BATCH_SIZE", 2000),
			ProgressEvery: getEnvInt("MIGRATOR_PROGRESS", 10000),
		},
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
