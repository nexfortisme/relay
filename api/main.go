package main

import (
	"log/slog"
	"os"

	"github.com/nexfortisme/relay/internal/app"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv, cleanup, err := app.NewServer(logger)
	if err != nil {
		logger.Error("failed to initialize server", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := srv.Run(); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}
