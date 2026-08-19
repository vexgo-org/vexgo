package main

import (
	"log"

	"vexgo/backend/internal/app"
	"vexgo/backend/internal/config"

	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.ParseFlags()

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	if err := application.Run(); err != nil {
		logrus.WithError(err).Fatal("Failed to start server")
	}
}
