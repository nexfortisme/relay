package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	LLMURL            string
	LLMModel          string
	SQLitePath        string
	DataDir           string
	WebOrigin         string
	MCPServerAddr     string
	MCPURL            string
	MaxUploadBytes    int64
	MaxImageBytes     int
	MaxTokenCount     int
	JWTSecret         string
	JWTRefreshSecret  string
	DisableAuth       bool
	RootUsername      string
	RootPassword      string
	CookieSecure      bool
}

func Load() (Config, error) {
	// load environment variables from .env file
	_ = godotenv.Overload(filepath.Join("..", ".env"))

	expandEnvKeys(
		"LLM_BASE_URL",
		"LLM_URL",
		"SEARXNG_URL",
		"MCP_SERVER_ADDRESS",
		"MCP_URL",
		"PLAYWRIGHT_MCP_ENDPOINT",
		"FETCHER_MCP_ENDPOINT",
	)

	cfg := Config{
		Port:           firstEnvWithDefault("8091", "API_PORT", "VITE_API_PORT"),
		LLMURL:         firstEnv("LLM_URL", "LLM_BASE_URL"),
		LLMModel:       envOrDefault("LLM_MODEL", "gpt-4o-mini"),
		SQLitePath:     envOrDefault("SQLITE_PATH", "../.relay/data/relay.db"),
		WebOrigin:      envOrDefault("WEB_ORIGIN", "http://localhost:5173"),
		MCPServerAddr:  envOrDefault("MCP_SERVER_ADDRESS", ":8090"),
		MCPURL:         envOrDefault("MCP_URL", "http://localhost:8090/mcp"),
		DataDir:        envOrDefault("DATA_DIR", "../.relay/data"),
		MaxUploadBytes: envInt64OrDefault("VITE_MAX_UPLOAD_BYTES", 50<<20),
		MaxImageBytes:  envIntOrDefault("VITE_MAX_IMAGE_BYTES", 15*1024*1024),
		MaxTokenCount:    envIntOrDefault("VITE_MAX_TOKEN_COUNT", 0),
		JWTSecret:        envOrDefault("JWT_TOKEN", "dev-insecure-jwt-secret-change-me"),
		JWTRefreshSecret: envOrDefault("JWT_REFRESH_TOKEN", "dev-insecure-refresh-secret-change-me"),
		DisableAuth:      envBoolOrDefault("DISABLE_AUTH", false),
		RootUsername:     envOrDefault("ROOT_USERNAME", "root"),
		RootPassword:     envOrDefault("ROOT_PASSWORD", "abc123"),
		CookieSecure:     envBoolOrDefault("COOKIE_SECURE", false),
	}

	if cfg.LLMURL == "" {
		return Config{}, errors.New("LLM_BASE_URL or LLM_URL must be set")
	}

	return cfg, nil
}

func envBoolOrDefault(key string, fallback bool) bool {
	val := strings.ToLower(sanitizeEnvValue(os.Getenv(key)))
	switch val {
	case "":
		return fallback
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	}
	return fallback
}

func (c Config) ListenAddr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func (c Config) NotebooksDir() string {
	return filepath.Join(c.DataDir, "notebooks")
}

func (c Config) SnapshotsDir(notebookID string) string {
	return filepath.Join(c.DataDir, "snapshots", notebookID)
}

func envOrDefault(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return sanitizeEnvValue(val)
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return sanitizeEnvValue(val)
		}
	}
	return ""
}

func firstEnvWithDefault(fallback string, keys ...string) string {
	if value := firstEnv(keys...); value != "" {
		return value
	}
	return fallback
}

func expandEnvKeys(keys ...string) {
	for _, key := range keys {
		val := os.Getenv(key)
		if val == "" {
			continue
		}
		_ = os.Setenv(key, os.ExpandEnv(val))
	}
}

func envIntOrDefault(key string, fallback int) int {
	val := sanitizeEnvValue(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envInt64OrDefault(key string, fallback int64) int64 {
	val := sanitizeEnvValue(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func sanitizeEnvValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if idx := strings.Index(trimmed, " #"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	return trimmed
}
