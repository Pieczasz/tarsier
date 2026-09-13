package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"

	"github.com/segmentio/kafka-go"
)

// The differentiator defect: the order is published with no trace context
func publishOrder(ctx context.Context, w *kafka.Writer, orderID string) error {
	return w.WriteMessages(ctx, kafka.Message{ // want: msgtrace/kafka-produce-no-inject
		Topic: "orders",
		Value: []byte(orderID),
	})
}

// Inline at the produce call with the propagator header set, so the Headers
// exclusion is the only thing keeping this clean.
func publishInstrumented(ctx context.Context, w *kafka.Writer, orderID string) error {
	return w.WriteMessages(ctx, kafka.Message{ // notwant: msgtrace/kafka-produce-no-inject
		Topic:   "orders",
		Value:   []byte(orderID),
		Headers: []kafka.Header{{Key: "traceparent", Value: []byte("00-abc-def-01")}},
	})
}

func buildRecord(orderID string) *kafka.Message {
	return &kafka.Message{Topic: "orders", Value: []byte(orderID)} // notwant: msgtrace/kafka-produce-no-inject
}

func publishViaVar(ctx context.Context, w *kafka.Writer, orderID string) error {
	msg := kafka.Message{Topic: "orders", Value: []byte(orderID)} // gap: msgtrace/kafka-produce-no-inject
	return w.WriteMessages(ctx, msg)
}

func notifyCustomer(orderID string) {
	log.Printf("notified customer for order %s", orderID) // want: logs/unstructured-logging
	slog.Info("customer notified", "order_id", orderID)
}

// No context, so no deadline, no cancellation, and no span propagation.
func fetchInventory(url string) (*http.Response, error) {
	return http.Get(url) // want: httpctx/outbound-call-without-context
}

func fetchInventoryOK(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // notwant: httpctx/outbound-call-without-context
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req) // notwant: httpctx/outbound-call-without-context
}

// The error is swallowed on a critical path
func reserveStock(ctx context.Context, sku string) {
	if err := reserve(ctx, sku); err != nil { // want: errors/swallowed-on-critical-path
		_ = err
	}
}

func reserveStockOK(ctx context.Context, sku string) error {
	if err := reserve(ctx, sku); err != nil { // notwant: errors/swallowed-on-critical-path
		return err
	}
	return nil
}
