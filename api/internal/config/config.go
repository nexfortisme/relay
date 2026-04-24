package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port       string
	LLMURL     string
	LLMModel   string
	SQLitePath string
	WebOrigin  string
}

func Load() (Config, error) {
	_ = godotenv.Overload(filepath.Join("..", ".env"))

	cfg := Config{
		Port:       envOrDefault("API_PORT", "8080"),
		LLMURL:     os.Getenv("LLM_URL"),
		LLMModel:   envOrDefault("LLM_MODEL", "gpt-4o-mini"),
		SQLitePath: envOrDefault("SQLITE_PATH", "relay.db"),
		WebOrigin:  envOrDefault("WEB_ORIGIN", "http://localhost:5173"),	
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
