package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	LLMURL         string
	LLMModel       string
	SQLitePath     string
	WebOrigin      string
	MCPServerAddr  string
	MCPURL         string
	MaxUploadBytes int64
	MaxImageBytes  int
	MaxTokenCount  int
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
		Port:           envOrDefault("VITE_API_PORT", "8090"),
		LLMURL:         firstEnv("LLM_URL", "LLM_BASE_URL"),
		LLMModel:       envOrDefault("LLM_MODEL", "gpt-4o-mini"),
		SQLitePath:     envOrDefault("SQLITE_PATH", "relay.db"),
		WebOrigin:      envOrDefault("WEB_ORIGIN", "http://localhost:5173"),
		MCPServerAddr:  envOrDefault("MCP_SERVER_ADDRESS", ":8090"),
		MCPURL:         envOrDefault("MCP_URL", "http://localhost:8090/mcp"),
		MaxUploadBytes: envInt64OrDefault("VITE_MAX_UPLOAD_BYTES", 50<<20),
		MaxImageBytes:  envIntOrDefault("VITE_MAX_IMAGE_BYTES", 15*1024*1024),
		MaxTokenCount:  envIntOrDefault("VITE_MAX_TOKEN_COUNT", 0),
	}

	fmt.Println("cfg", cfg)

	if cfg.LLMURL == "" {
		return Config{}, errors.New("LLM_BASE_URL or LLM_URL must be set")
	}

	return cfg, nil
}

func (c Config) ListenAddr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func envOrDefault(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return ""
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
	val := os.Getenv(key)
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
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
