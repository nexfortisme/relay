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
	RelayDir       string
	MaxUploadBytes int64
	MaxImageBytes  int
}

func Load() (Config, error) {
	_ = godotenv.Overload(filepath.Join("..", ".env"))

	cfg := Config{
		Port:           envOrDefault("API_PORT", "8080"),
		LLMURL:         os.Getenv("LLM_URL"),
		LLMModel:       envOrDefault("LLM_MODEL", "gpt-4o-mini"),
		SQLitePath:     envOrDefault("SQLITE_PATH", "relay.db"),
		WebOrigin:      envOrDefault("WEB_ORIGIN", "http://localhost:5173"),
		RelayDir:       envOrDefault("RELAY_DIR", filepath.Join("..", ".relay")),
		MaxUploadBytes: envInt64OrDefault("MAX_UPLOAD_BYTES", 50<<20),
		MaxImageBytes:  envIntOrDefault("MAX_IMAGE_BYTES", 15*1024*1024),
	}

	if cfg.LLMURL == "" {
		return Config{}, errors.New("LLM_URL must be set")
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
