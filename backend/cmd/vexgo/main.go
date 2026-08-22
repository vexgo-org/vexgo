// Command vexgo is the VexGo server entry point. It parses configuration,
// wires the application via the app package, and serves HTTP until shutdown.
package main

import (
	"log/slog"
	"os"

	"github.com/vexgo-org/vexgo/backend/internal/app"
	"github.com/vexgo-org/vexgo/backend/internal/config"
)

func main() {
	cfg := config.ParseFlags()

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("failed to initialize application", "err", err)
		os.Exit(1)
	}

	if err := application.Run(); err != nil {
		slog.Error("failed to start server", "err", err)
		os.Exit(1)
	}
}
