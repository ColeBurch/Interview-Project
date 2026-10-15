package main

import (
	"log/slog"
	"net/http"
	"os"
)

type app struct {
	logger   *slog.Logger
	presence *presence
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true}))

	app := &app{
		logger:   logger,
		presence: newPresence(),
	}

	srv := &http.Server{
		Addr:     ":8000",
		Handler:  app.routes(),
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "addr", ":8000")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
