// Command vexgo is the VexGo server entry point. It parses configuration,
// wires the application via the app package, and serves HTTP until shutdown.
package main

import (
	"vexgo/backend/internal/app"
	"vexgo/backend/internal/config"

	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.ParseFlags()

	application, err := app.New(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize application")
	}

	if err := application.Run(); err != nil {
		logrus.WithError(err).Fatal("Failed to start server")
	}
}
