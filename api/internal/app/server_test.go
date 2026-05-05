package app

import (
	"io"
	"log/slog"
	"testing"

	"github.com/nexfortisme/relay/internal/config"
)

func TestNewServerWithFeedsRoutes(t *testing.T) {
	server, cleanup, err := NewServerWithConfig(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{
		Port:             "0",
		LLMURL:           "http://localhost:1/v1",
		LLMModel:         "test-model",
		SQLitePath:       t.TempDir() + "/relay.db",
		WebOrigin:        "http://localhost:5173",
		MCPURL:           "http://localhost:8090/mcp",
		MaxUploadBytes:   50 << 20,
		MaxImageBytes:    15 << 20,
		JWTSecret:        "test-access-secret",
		JWTRefreshSecret: "test-refresh-secret",
		DisableAuth:      true,
		RootUsername:     "root",
		RootPassword:     "abc123",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer cleanup()
	if server == nil || server.engine == nil {
		t.Fatal("expected initialized server")
	}
}
