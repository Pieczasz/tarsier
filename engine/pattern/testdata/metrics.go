package testdata

import "github.com/prometheus/client_golang/prometheus"

// Two different metrics, both carrying the same unbounded label: the case that
// collapses into one fingerprint if the driver has no discriminator.
var orders = prometheus.NewCounterVec(
	prometheus.CounterOpts{Name: "orders_total"},
	[]string{"user_id", "status"},
)

var exports = prometheus.NewCounterVec(
	prometheus.CounterOpts{Name: "exports_total"},
	[]string{"user_id"},
)

var fine = prometheus.NewCounterVec(
	prometheus.CounterOpts{Name: "ok_total"},
	[]string{"status", "region"},
)

var notAMetric = []string{"user_id"}
