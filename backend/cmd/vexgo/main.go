// Command vexgo is the VexGo server entry point. It resolves configuration
// via the cli package, wires the application through the app package, and
// serves HTTP until shutdown.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/vexgo-org/vexgo/backend/internal/app"
	"github.com/vexgo-org/vexgo/backend/internal/cli"
)

// Version is the build version, overridable at build time via ldflags:
//
//	go build -ldflags "-X main.Version=1.2.3"
var Version = "dev"

func main() {
	cfg, err := cli.Execute(Version, os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "vexgo: error: %v\n", err)
		os.Exit(2)
	}
	if cfg == nil {
		// Help or version information was printed; nothing to run.
		return
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
