package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func checkout(ctx context.Context) error {
	ctx, span := otel.Tracer("api").Start(ctx, "checkout")
	defer span.End()
	if err := reserve(ctx, "sku"); err != nil { // want: traces/error-path-not-recorded-on-span
		return err
	}
	return nil
}

func checkoutOK(ctx context.Context) error {
	ctx, span := otel.Tracer("api").Start(ctx, "checkout")
	defer span.End()
	if err := reserve(ctx, "sku"); err != nil { // notwant: traces/error-path-not-recorded-on-span
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}
