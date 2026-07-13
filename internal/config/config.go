package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv          string
	HTTPAddr        string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	Database        DatabaseConfig
	Auth            AuthConfig
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type AuthConfig struct {
	PasswordBcryptCost  int
	AccessTokenIssuer   string
	AccessTokenAudience string
	AccessTokenSecret   string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
}

func Load() Config {
	return Config{
		AppEnv:          getEnv("APP_ENV", "local"),
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     60 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		Database: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", "postgres://corebe:corebe_dev_password@localhost:5432/corebe?sslmode=disable"),
			MaxConns:        getEnvInt32("DB_MAX_CONNS", 25),
			MinConns:        getEnvInt32("DB_MIN_CONNS", 2),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Auth: AuthConfig{
			PasswordBcryptCost:  getEnvInt("AUTH_PASSWORD_BCRYPT_COST", 12),
			AccessTokenIssuer:   getEnv("AUTH_ACCESS_TOKEN_ISSUER", "corebe-api"),
			AccessTokenAudience: getEnv("AUTH_ACCESS_TOKEN_AUDIENCE", "corebe-api"),
			AccessTokenSecret:   getEnv("AUTH_ACCESS_TOKEN_SECRET", ""),
			AccessTokenTTL:      getEnvDuration("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL:     getEnvDuration("AUTH_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		},
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt32(key string, fallback int32) int32 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(parsed)
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
