package godeep

import (
	"context"

	"go.opentelemetry.io/otel/sdk/resource"
)

// NewResource is a one-hop wrapper around resource.New without service.name.
func NewResource(ctx context.Context) (*resource.Resource, error) {
	return resource.New(ctx) // want-deep: otel/sdk-missing-resource-attrs
}

func Boot(ctx context.Context) error {
	_, err := NewResource(ctx) // want-deep: otel/sdk-missing-resource-attrs
	return err
}
