package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// A metric declared in a test is not production instrumentation
func TestOrderCounter(t *testing.T) {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_orders_total",
	}, []string{
		"user_id", // notwant: metrics/high-cardinality-label
	})
	_ = c
}
