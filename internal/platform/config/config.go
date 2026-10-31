// Package config carga la configuración desde variables de entorno, siguiendo
// la metodología 12-Factor App (https://12factor.net/config).
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config es la configuración de la aplicación.
type Config struct {
	Addr            string        // INVENTORY_ADDR
	DBPath          string        // INVENTORY_DB_PATH
	LogLevel        slog.Level    // INVENTORY_LOG_LEVEL (debug|info|warn|error)
	LogFormat       string        // INVENTORY_LOG_FORMAT (text|json)
	Seed            bool          // INVENTORY_SEED: carga datos de demostración si la BD está vacía
	ShutdownTimeout time.Duration // INVENTORY_SHUTDOWN_TIMEOUT
}

// Load lee y valida la configuración. getenv se inyecta para poder probarla.
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg := Config{
		Addr:            withDefault(getenv("INVENTORY_ADDR"), ":8080"),
		DBPath:          withDefault(getenv("INVENTORY_DB_PATH"), "inventory.db"),
		LogFormat:       strings.ToLower(withDefault(getenv("INVENTORY_LOG_FORMAT"), "text")),
		ShutdownTimeout: 10 * time.Second,
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(withDefault(getenv("INVENTORY_LOG_LEVEL"), "info"))); err != nil {
		return cfg, fmt.Errorf("INVENTORY_LOG_LEVEL: %w", err)
	}
	if cfg.LogFormat != "text" && cfg.LogFormat != "json" {
		return cfg, fmt.Errorf("INVENTORY_LOG_FORMAT must be text or json, got %q", cfg.LogFormat)
	}
	if v := getenv("INVENTORY_SEED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("INVENTORY_SEED: %w", err)
		}
		cfg.Seed = b
	}
	if v := getenv("INVENTORY_SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return cfg, fmt.Errorf("INVENTORY_SHUTDOWN_TIMEOUT must be a positive duration, got %q", v)
		}
		cfg.ShutdownTimeout = d
	}
	return cfg, nil
}

func withDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
