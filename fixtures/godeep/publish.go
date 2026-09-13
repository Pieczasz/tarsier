package godeep

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// Publish wraps WriteMessages so the pattern tier cannot see call sites.
func Publish(ctx context.Context, w *kafka.Writer, msg kafka.Message) error {
	return w.WriteMessages(ctx, msg)
}

// Checkout is the wrapper-follow proof: pattern misses; deep msgtrace hits.
func Checkout(ctx context.Context, w *kafka.Writer, orderID string) error {
	return Publish(ctx, w, kafka.Message{ // want-deep: msgtrace/kafka-produce-no-inject
		Topic: "orders",
		Value: []byte(orderID),
	})
}

func CheckoutOK(ctx context.Context, w *kafka.Writer, orderID string) error {
	return Publish(ctx, w, kafka.Message{ // notwant-deep: msgtrace/kafka-produce-no-inject
		Topic:   "orders",
		Value:   []byte(orderID),
		Headers: []kafka.Header{{Key: "traceparent", Value: []byte("00-a-b-01")}},
	})
}
