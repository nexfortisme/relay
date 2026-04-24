package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nexfortisme/relay/internal/app"
	"github.com/nexfortisme/relay/internal/config"
	internalmcp "github.com/nexfortisme/relay/internal/mcp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	srv, cleanup, err := app.NewServerWithConfig(logger, cfg)
	if err != nil {
		logger.Error("failed to initialize server", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	mcpServer := internalmcp.NewServer(cfg.MCPServerAddr, logger)
	errCh := make(chan error, 2)

	go func() {
		errCh <- srv.Run()
	}()

	go func() {
		errCh <- mcpServer.Start()
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(interrupt)

	select {
	case sig := <-interrupt:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server exited", "error", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mcpServer.Shutdown(ctx); err != nil {
		logger.Error("failed to stop mcp server", "error", err)
	}
}
