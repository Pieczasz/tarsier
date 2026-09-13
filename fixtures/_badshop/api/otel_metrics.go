package main

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTel metrics attributes: same denylist as prometheus label names, gated on
// the attribute import so ordinary String() helpers do not fire (n8n lesson).
func recordOTelCheckout(ctx context.Context, c metric.Int64Counter, userID, status string) {
	c.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user_id", userID), // want: metrics/high-cardinality-label
		attribute.String("status", status),  // notwant: metrics/high-cardinality-label
	))
}
