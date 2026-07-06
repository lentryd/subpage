// Package config holds runtime configuration shared across the app.
package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Config is passed into api.New and into every handler that needs it
// (DB pool, VPN node registry, etc. would live here too).
type Config struct {
	Port  string
	NoWeb bool
	Debug bool

	// Remnawave panel connection.
	RemnawavePanelURL string
	RemnawaveAPIToken string
	SubpageConfigUUID string
	CustomSubPrefix   string
	TrustProxy        string
	InternalJWTSecret string
}

func New(port string, noWeb, debug bool) *Config {
	cfg := &Config{
		Port:  port,
		NoWeb: noWeb,
		Debug: debug,

		RemnawavePanelURL: os.Getenv("REMNAWAVE_PANEL_URL"),
		RemnawaveAPIToken: os.Getenv("REMNAWAVE_API_TOKEN"),
		SubpageConfigUUID: getenvDefault("SUBPAGE_CONFIG_UUID", "00000000-0000-0000-0000-000000000000"),
		CustomSubPrefix:   os.Getenv("CUSTOM_SUB_PREFIX"),
		TrustProxy:        getenvDefault("TRUST_PROXY", "1"),
		InternalJWTSecret: os.Getenv("INTERNAL_JWT_SECRET"),
	}

	if err := cfg.validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	return cfg
}

func (c *Config) validate() error {
	if c.RemnawavePanelURL == "" {
		return fmt.Errorf("REMNAWAVE_PANEL_URL is required")
	}
	if !strings.HasPrefix(c.RemnawavePanelURL, "http://") && !strings.HasPrefix(c.RemnawavePanelURL, "https://") {
		return fmt.Errorf("REMNAWAVE_PANEL_URL must start with http:// or https://")
	}
	if c.RemnawaveAPIToken == "" {
		return fmt.Errorf("REMNAWAVE_API_TOKEN is required")
	}
	if c.InternalJWTSecret == "" {
		return fmt.Errorf("INTERNAL_JWT_SECRET is required")
	}
	return nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
