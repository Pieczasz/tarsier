package main

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// The differentiator defect: consumed messages never join the producer trace.
func consumeOrders(ctx context.Context, r *kafka.Reader) error {
	for {
		m, err := r.ReadMessage(ctx) // want: msgtrace/kafka-consume-no-extract
		if err != nil {
			return err
		}
		_ = m.Value
	}
}

// Extraction in the enclosing block is the only thing keeping this clean.
func consumeInstrumented(ctx context.Context, r *kafka.Reader) error {
	for {
		m, err := r.ReadMessage(ctx) // notwant: msgtrace/kafka-consume-no-extract
		if err != nil {
			return err
		}
		ctx = extractTrace(ctx, m.Headers)
		_ = ctx
	}
}

func extractTrace(ctx context.Context, _ []kafka.Header) context.Context { return ctx }

type consumer struct{ reader *kafka.Reader }

// Delegation to a same-named local wrapper is not a client read: client
// reads take exactly ctx (crowdsec pkg/acquisition/modules/kafka/run.go:66).
func (c *consumer) ReadMessage(ctx context.Context, out chan<- string) error {
	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		ctx = extractTrace(ctx, m.Headers)
		out <- string(m.Value)
	}
}

func (c *consumer) run(ctx context.Context, out chan<- string) error {
	return c.ReadMessage(ctx, out) // notwant: msgtrace/kafka-consume-no-extract
}
