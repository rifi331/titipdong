// Package config loads TitipDong's runtime configuration from environment.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime settings.
type Config struct {
	Addr          string // HTTP listen address
	DatabaseURL   string // postgres://...
	SessionSecret string // 32+ bytes, used to sign session cookies
	UploadsDir    string // where item/KTP/receipt photos live
	BaseURL       string // used for absolute URLs (PWA manifest, etc.)

	// Admin bootstrap (only used on first boot if no admin exists).
	AdminEmail    string
	AdminPassword string

	// Optional receipt-scan via OpenAI vision. Empty => feature hidden.
	OpenAIAPIKey string

	// FX source. Default frankfurter.app (free, no key).
	FXBaseURL string
}

// Load reads config from environment, applying defaults.
func Load() (Config, error) {
	c := Config{
		Addr:          env("ADDR", ":8080"),
		DatabaseURL:   env("DATABASE_URL", "postgres://titipdong:titipdong@localhost:5432/titipdong?sslmode=disable"),
		SessionSecret: env("SESSION_SECRET", ""),
		UploadsDir:    env("UPLOADS_DIR", "./uploads"),
		BaseURL:       strings.TrimRight(env("BASE_URL", "http://localhost:8080"), "/"),
		AdminEmail:    env("ADMIN_EMAIL", ""),
		AdminPassword: env("ADMIN_PASSWORD", ""),
		OpenAIAPIKey:  env("OPENAI_API_KEY", ""),
		FXBaseURL:     strings.TrimRight(env("FX_BASE_URL", "https://api.frankfurter.app"), "/"),
	}

	if c.SessionSecret == "" {
		// Allow dev convenience but warn in prod by checking length later.
		c.SessionSecret = "dev-insecure-secret-change-me-in-production"
	}
	if len(c.SessionSecret) < 32 && os.Getenv("SESSION_SECRET") != "" {
		return c, fmt.Errorf("SESSION_SECRET must be at least 32 bytes")
	}
	return c, nil
}

// HasOpenAI reports whether receipt scanning is enabled.
func (c Config) HasOpenAI() bool { return c.OpenAIAPIKey != "" }

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
