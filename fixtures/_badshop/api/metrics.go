package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// one time series per customer
	ordersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "badshop_orders_total",
		Help: "Orders placed.",
	}, []string{
		"user_id", // want: metrics/high-cardinality-label
		"status",
	})

	checkoutDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "badshop_checkout_duration_seconds",
		Help: "Checkout latency.",
	}, []string{
		"email", // want: metrics/high-cardinality-label
		"region",
	})

	// tenant_id is bounded and is usually the label you actually want
	tenantRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "badshop_tenant_requests_total",
		Help: "Requests per tenant.",
	}, []string{
		"tenant_id", // notwant: metrics/high-cardinality-label
	})

	// across the Grafana stack "user" means tenant
	queueLength = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "badshop_queue_length",
		Help: "Queued requests per tenant.",
	}, []string{
		"user", // notwant: metrics/high-cardinality-label
	})
)

// just a slice of strings
var auditFields = []string{
	"user_id", // notwant: metrics/high-cardinality-label
	"action",
}

// The other end of the metric: the label is named "error", which passes any
// name-based denylist, but the value is an error string full of addresses,
// ports and timings.
func recordFailure(err error, status string) {
	ordersFailed.WithLabelValues(err.Error()).Inc() // want: metrics/unbounded-label-value
	ordersFailed.WithLabelValues(status).Inc()      // notwant: metrics/unbounded-label-value
	ordersFailed.WithLabelValues("timeout").Inc()   // notwant: metrics/unbounded-label-value
}
