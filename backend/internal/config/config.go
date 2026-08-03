package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv string
	Port   string

	DBDSN string

	JWTSecret              string
	JWTExpiryMinutes       int
	RefreshTokenExpiryDays int

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string

	SMSProvider string
	SMSAPIKey   string

	BaseURL string
}

// Load reads backend/.env (if present, without overriding already-set env vars)
// and builds a Config from the process environment.
func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		Port:        getEnv("PORT", "8080"),
		DBDSN:       os.Getenv("DB_DSN"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		SMTPHost:    os.Getenv("SMTP_HOST"),
		SMTPPort:    getEnv("SMTP_PORT", "587"),
		SMTPUser:    os.Getenv("SMTP_USER"),
		SMTPPass:    os.Getenv("SMTP_PASS"),
		SMTPFrom:    getEnv("SMTP_FROM", "noreply@clinic.local"),
		SMSProvider: os.Getenv("SMS_PROVIDER"),
		SMSAPIKey:   os.Getenv("SMS_API_KEY"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
	}

	jwtExpiry, err := strconv.Atoi(getEnv("JWT_EXPIRY_MINUTES", "15"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY_MINUTES: %w", err)
	}
	cfg.JWTExpiryMinutes = jwtExpiry

	refreshExpiry, err := strconv.Atoi(getEnv("REFRESH_TOKEN_EXPIRY_DAYS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TOKEN_EXPIRY_DAYS: %w", err)
	}
	cfg.RefreshTokenExpiryDays = refreshExpiry

	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv sets process env vars from a simple KEY=VALUE file, one per line.
// Existing env vars are never overridden. Missing file is not an error.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
