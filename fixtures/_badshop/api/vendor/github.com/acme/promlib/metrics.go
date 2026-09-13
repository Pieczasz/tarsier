package promlib

import "github.com/prometheus/client_golang/prometheus"

// Vendored third-party code.
var Requests = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "promlib_requests_total",
}, []string{
	"user_id", // notwant: metrics/high-cardinality-label
})
