package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Scenario: load a TOML timeout, then set its environment override; expect the override to take precedence.
func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[auth_check]\ntimeout = \"not-a-duration\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AUTH_CHECK_TIMEOUT", "")
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.GetString("auth_check.timeout"); got != "not-a-duration" {
		t.Fatalf("config timeout = %q, want file value", got)
	}

	t.Setenv("AUTH_CHECK_TIMEOUT", "7s")
	if got := config.GetString("auth_check.timeout"); got != "7s" {
		t.Fatalf("config timeout = %q, want environment override", got)
	}
}

// Scenario: load a missing TOML file; expect an error that identifies the missing path.
func TestLoadConfigMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	if _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("LoadConfig() error = %v, want file path", err)
	}
}

// Scenario: read the cache cleanup interval from TOML and then from the environment; expect both values in seconds.
func TestAuthCacheCleanupIntervalConfig(t *testing.T) {
	t.Setenv("AUTH_CACHE_CLEANUP_INTERVAL", "")
	config, err := LoadConfig(filepath.Join("..", "..", "configs", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := time.Duration(config.GetInt("auth_cache.cleanup_interval")) * time.Second; got != 5*time.Minute {
		t.Fatalf("cleanup interval = %s, want 5m", got)
	}

	t.Setenv("AUTH_CACHE_CLEANUP_INTERVAL", "5")
	if got := time.Duration(config.GetInt("auth_cache.cleanup_interval")) * time.Second; got != 5*time.Second {
		t.Fatalf("environment cleanup interval = %s, want 5s", got)
	}
}
