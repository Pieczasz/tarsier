package main

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func bootTracer(ctx context.Context) {
	_ = sdktrace.NewTracerProvider() // want: otel/sdk-missing-shutdown
}

func bootTracerOK(ctx context.Context) {
	tp := sdktrace.NewTracerProvider() // notwant: otel/sdk-missing-shutdown
	defer tp.Shutdown(ctx)
}
