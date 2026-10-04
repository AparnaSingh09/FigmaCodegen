// Package config loads the MCP server's configuration from two places,
// same split used in commit-notification-app and for the same reason:
//
//   - .env: secrets (API keys/tokens). Never committed - see .gitignore.
//   - config.json: everything else (model names, defaults, safety caps).
//     Safe to commit; easier to review than scattered env vars.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config is the fully-resolved configuration the rest of the app reads from.
type Config struct {
	// Secrets, from .env.
	FigmaToken   string
	GeminiAPIKey string

	// Settings, from config.json.
	GeminiModel      string
	DefaultFramework string
	MaxImageBytes    int
}

// fileSettings mirrors config.json's shape.
type fileSettings struct {
	GeminiModel      string `json:"geminiModel"`
	DefaultFramework string `json:"defaultFramework"`
	MaxImageBytes    int    `json:"maxImageBytes"`
}

func defaultFileSettings() fileSettings {
	return fileSettings{
		GeminiModel:      "gemini-2.5-flash",
		DefaultFramework: "react",
		MaxImageBytes:    5_000_000, // 5MB - Figma frame PNGs are usually well under this
	}
}

// Load reads .env (if present) and config.json (if present), falling back to
// defaults for anything config.json omits, and returns the resolved Config.
// It does not error if .env is missing (e.g. in a container where secrets
// arrive as real env vars instead) or if config.json is missing (defaults
// apply) - it only errors if config.json exists but fails to parse, since
// that's a real mistake worth surfacing rather than silently ignoring.
func Load() (*Config, error) {
	if envPath, err := findFileUpward(".env"); err == nil {
		_ = godotenv.Load(envPath) // best-effort; real env vars still win
	}

	settings := defaultFileSettings()
	if cfgPath, err := findFileUpward("config.json"); err == nil {
		loaded, err := loadFileSettings(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		settings = loaded
	}

	return &Config{
		FigmaToken:       os.Getenv("FIGMA_TOKEN"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		GeminiModel:      settings.GeminiModel,
		DefaultFramework: settings.DefaultFramework,
		MaxImageBytes:    settings.MaxImageBytes,
	}, nil
}

func loadFileSettings(path string) (fileSettings, error) {
	settings := defaultFileSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}
	return settings, nil
}

// findFileUpward looks for filename in the current directory, then each
// parent directory up to the filesystem root. This lets the server be run
// from cmd/server/ (go run ./cmd/server) or from mcp-server/ itself and
// still find config.json/.env sitting at the module root.
func findFileUpward(filename string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found", filename)
		}
		dir = parent
	}
}
