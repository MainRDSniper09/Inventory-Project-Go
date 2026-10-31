package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/MainRDSniper09/Inventory-Project-Go/internal/platform/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := config.Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.DBPath != "inventory.db" || cfg.LogLevel != slog.LevelInfo ||
		cfg.LogFormat != "text" || cfg.Seed || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := config.Load(env(map[string]string{
		"INVENTORY_ADDR": ":9000", "INVENTORY_DB_PATH": "/data/x.db", "INVENTORY_LOG_LEVEL": "debug",
		"INVENTORY_LOG_FORMAT": "JSON", "INVENTORY_SEED": "true", "INVENTORY_SHUTDOWN_TIMEOUT": "3s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9000" || cfg.DBPath != "/data/x.db" || cfg.LogLevel != slog.LevelDebug ||
		cfg.LogFormat != "json" || !cfg.Seed || cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("unexpected overrides: %+v", cfg)
	}
}

func TestLoad_Invalid(t *testing.T) {
	for _, m := range []map[string]string{
		{"INVENTORY_LOG_LEVEL": "loud"},
		{"INVENTORY_LOG_FORMAT": "xml"},
		{"INVENTORY_SEED": "maybe"},
		{"INVENTORY_SHUTDOWN_TIMEOUT": "-1s"},
	} {
		if _, err := config.Load(env(m)); err == nil {
			t.Errorf("expected error for %v", m)
		}
	}
}
