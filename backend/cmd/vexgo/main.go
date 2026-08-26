// Command vexgo is the VexGo server entry point. It parses configuration,
// wires the application via the app package, and serves HTTP until shutdown.
package main

import (
	"log/slog"
	"os"

	"github.com/vexgo-org/vexgo/backend/internal/app"
	"github.com/vexgo-org/vexgo/backend/internal/config"
)

// Version is the build version, overridable at build time via ldflags:
//
//	go build -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	action, cfg := config.ParseFlags(Version, os.Args[1:])

	switch action {
	case config.ActionHelp:
		config.PrintUsage()
		os.Exit(0)
	case config.ActionVersion:
		// Version already printed by ParseFlags.
		os.Exit(0)
	case config.ActionRun:
		if cfg == nil {
			os.Exit(2)
		}
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
}
