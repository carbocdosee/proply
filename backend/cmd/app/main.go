package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"proply/internal/app"
	"proply/internal/config"
)

func main() {
	// Load .env file in development (no-op if file not found)
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	a, err := app.New(cfg)
	if err != nil {
		slog.Error("app init failed", "error", err)
		os.Exit(1)
	}

	if err := a.Run(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
