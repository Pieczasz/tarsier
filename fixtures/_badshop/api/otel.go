package main

import (
	"context"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func initResource(ctx context.Context) {
	resource.New(ctx) // want: otel/sdk-missing-resource-attrs
}

func initResourceOK(ctx context.Context) {
	resource.New(ctx, resource.WithAttributes(semconv.ServiceName("api"))) // notwant: otel/sdk-missing-resource-attrs
}

func initResourceFromEnv(ctx context.Context) {
	resource.New(ctx, resource.WithFromEnv()) // notwant: otel/sdk-missing-resource-attrs
}
