package main

import (
	"context"
	"fmt"
	"log/slog"
)

func logOrderFailed(id string, err error) {
	slog.Info(fmt.Sprintf("order %s failed: %v", id, err)) // want: logs/slog-sprintf-message
}

func logBoom(ctx context.Context, id string) {
	slog.ErrorContext(ctx, fmt.Sprintf("boom %s", id)) // want: logs/slog-sprintf-message
}

func logStructured(id string) {
	slog.Info("order failed", "order_id", fmt.Sprintf("%s", id)) // notwant: logs/slog-sprintf-message
}

func logStructuredCtx(ctx context.Context, id string) {
	slog.ErrorContext(ctx, "boom", "id", fmt.Sprintf("%s", id)) // notwant: logs/slog-sprintf-message
}
