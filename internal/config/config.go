// Package config loads settings from environment variables (and an optional
// .env file in development).
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
	LogLevel    slog.Level
	LogFormat   string   // "json" or "text"
	CORSOrigins []string // allowed origins; empty disables CORS, "*" allows any

	RateLimitRPS     int    // requests/second per client IP, all routes; 0 disables
	AuthRateLimitRPM int    // requests/minute per client IP on /api/v1/auth/*; 0 disables
	ClientIPHeader   string // header your proxy sets to the client IP; empty uses the TCP peer
}

// Load reads the config and reports every invalid or missing value at once.
func Load() (Config, error) {
	_ = godotenv.Load() // .env is optional; real env vars take precedence

	cfg := Config{
		HTTPAddr:       getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		LogFormat:      getenv("LOG_FORMAT", "json"),
		CORSOrigins:    strings.FieldsFunc(os.Getenv("CORS_ALLOWED_ORIGINS"), func(r rune) bool { return r == ',' || r == ' ' }),
		ClientIPHeader: os.Getenv("CLIENT_IP_HEADER"),
	}

	var errs []error
	var err error
	count := func(key string, fallback int) int {
		n, err := strconv.Atoi(getenv(key, strconv.Itoa(fallback)))
		if err != nil || n < 0 {
			errs = append(errs, fmt.Errorf("%s must be a whole number >= 0", key))
		}
		return n
	}
	cfg.RateLimitRPS = count("RATE_LIMIT_RPS", 20)
	cfg.AuthRateLimitRPM = count("AUTH_RATE_LIMIT_RPM", 10)
	if cfg.JWTTTL, err = time.ParseDuration(getenv("JWT_TTL", "24h")); err != nil {
		errs = append(errs, fmt.Errorf("JWT_TTL: %w", err))
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		errs = append(errs, errors.New("LOG_FORMAT must be json or text"))
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(cfg.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 characters"))
	}
	return cfg, errors.Join(errs...)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
