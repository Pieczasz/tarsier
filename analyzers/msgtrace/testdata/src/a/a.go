package a

import (
	"context"

	"github.com/IBM/sarama"
	"github.com/segmentio/kafka-go"
	"github.com/twmb/franz-go/pkg/kgo"
)

func Publish(ctx context.Context, w *kafka.Writer, msg kafka.Message) error {
	return w.WriteMessages(ctx, msg)
}

type Bus struct{}

func (b *Bus) Publish(ctx context.Context, w *kafka.Writer, msg kafka.Message) error {
	return w.WriteMessages(ctx, msg)
}

func bad(ctx context.Context, w *kafka.Writer) {
	_ = Publish(ctx, w, kafka.Message{ // want "Message published via wrapper with no Headers"
		Topic: "t",
		Value: []byte("x"),
	})
}

func badMethod(ctx context.Context, w *kafka.Writer, b *Bus) {
	_ = b.Publish(ctx, w, kafka.Message{ // want "Message published via wrapper with no Headers"
		Topic: "t",
		Value: []byte("x"),
	})
}

func ok(ctx context.Context, w *kafka.Writer) {
	_ = Publish(ctx, w, kafka.Message{
		Topic:   "t",
		Value:   []byte("x"),
		Headers: []kafka.Header{{Key: "traceparent", Value: []byte("00")}},
	})
}

func direct(ctx context.Context, w *kafka.Writer) {
	_ = w.WriteMessages(ctx, kafka.Message{ // want "Message published with no Headers"
		Topic: "t",
		Value: []byte("x"),
	})
}

func unkeyed(ctx context.Context, w *kafka.Writer) {
	// Unkeyed elts are not KeyValueExpr; hasHeaders must skip them.
	_ = w.WriteMessages(ctx, kafka.Message{"t", []byte("x"), nil}) // want "Message published with no Headers"
}

func notWrapper(ctx context.Context, w *kafka.Writer, s string) {
	_ = w.WriteMessages(ctx, kafka.Message{Topic: "t", Value: []byte(s)}) // want "Message published with no Headers"
}

func saramaDirect(p *sarama.SyncProducer) {
	_, _, _ = p.SendMessage(sarama.ProducerMessage{ // want "Message published with no Headers"
		Topic: "t",
		Value: "x",
	})
}

func kgoDirect(ctx context.Context, c *kgo.Client) {
	_ = c.ProduceSync(ctx, kgo.Record{ // want "Message published with no Headers"
		Topic: "t",
		Value: []byte("x"),
	})
}

func other(ctx context.Context) {}
