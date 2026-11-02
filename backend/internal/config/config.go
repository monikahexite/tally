// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Env            string // "development" or "production"
	Port           string
	MongoURI       string
	MongoDB        string
	RedisURL       string
	JWTSecret      []byte
	SessionTTL     time.Duration
	CookieSecure   bool
	StaticDir      string   // built React app; empty = API only
	TrustedProxies []string // CIDRs whose X-Forwarded-For we believe
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

func Load() (*Config, error) {
	cfg := &Config{
		Env:        getenv("APP_ENV", "development"),
		Port:       getenv("PORT", "8080"),
		MongoURI:   os.Getenv("MONGO_URI"),
		MongoDB:    getenv("MONGO_DB", "livepoll"),
		RedisURL:   os.Getenv("REDIS_URL"),
		JWTSecret:  []byte(os.Getenv("JWT_SECRET")),
		SessionTTL: 7 * 24 * time.Hour,
		StaticDir:  os.Getenv("STATIC_DIR"),
		// Private ranges: platform load balancers (Render, Railway, Fly...) live here.
		TrustedProxies: splitList(getenv("TRUSTED_PROXIES",
			"10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.1/32,::1/128")),
	}
	cfg.CookieSecure = getenv("COOKIE_SECURE", boolStr(cfg.IsProduction())) == "true"

	var errs []error
	if cfg.MongoURI == "" {
		errs = append(errs, errors.New("MONGO_URI is required"))
	}
	if cfg.RedisURL == "" {
		errs = append(errs, errors.New("REDIS_URL is required"))
	}
	if len(cfg.JWTSecret) < 32 {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(cfg.JWTSecret)))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
