package main

import (
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
)

func setupLogger() *slog.Logger {
	_ = otel.GetTracerProvider()
	return slog.New(slog.NewJSONHandler(os.Stdout, nil)) // want: logs/missing-trace-correlation
}
