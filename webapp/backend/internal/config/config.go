// Package config loads this backend's settings, same .env/config.json
// split used everywhere else in this repo. This service holds no API
// keys of its own - those live in mcp-server/.env - so .env here only
// carries PORT, kept for consistency with how commit-notification-app
// treats PORT (an env var, even though it's not a secret).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	FrontendURL   string
	MCPServerPath string
}

type fileSettings struct {
	MCPServerPath string `json:"mcpServerPath"`
}

func defaultFileSettings() fileSettings {
	return fileSettings{
		// Default assumes this backend is run from webapp/backend/ and
		// mcp-server/ is a sibling of webapp/ - see README for the layout.
		MCPServerPath: "../../mcp-server/server",
	}
}

func Load() (*Config, error) {
	if envPath, err := findFileUpward(".env"); err == nil {
		_ = godotenv.Load(envPath)
	}

	settings := defaultFileSettings()
	if cfgPath, err := findFileUpward("config.json"); err == nil {
		loaded, err := loadFileSettings(cfgPath)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		settings = loaded
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	return &Config{
		Port:          port,
		FrontendURL:   frontendURL,
		MCPServerPath: settings.MCPServerPath,
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
