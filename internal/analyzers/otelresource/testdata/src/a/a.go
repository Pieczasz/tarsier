package a

import (
	"context"

	otelresource "go.opentelemetry.io/otel/sdk/resource"
)

func NewResource(ctx context.Context) (*otelresource.Resource, error) {
	return otelresource.New(ctx) // want "OTel resource is created without service.name"
}

func NewOK(ctx context.Context) (*otelresource.Resource, error) {
	return otelresource.New(ctx, otelresource.WithAttributes(otelresource.ServiceName("api")))
}

func NewFromEnv(ctx context.Context) (*otelresource.Resource, error) {
	return otelresource.New(ctx, otelresource.WithFromEnv())
}

type Factory struct{}

func (f *Factory) NewResource(ctx context.Context) (*otelresource.Resource, error) {
	return otelresource.New(ctx) // want "OTel resource is created without service.name"
}

func boot(ctx context.Context) {
	_, _ = NewResource(ctx) // want "wrapper constructs an OTel resource without service.name"
}

func bootMethod(ctx context.Context, f *Factory) {
	_, _ = f.NewResource(ctx) // want "wrapper constructs an OTel resource without service.name"
}
