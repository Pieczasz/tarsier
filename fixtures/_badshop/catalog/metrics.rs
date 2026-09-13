use prometheus::{register_counter_vec, CounterVec, Opts};

pub fn register_lookups() -> CounterVec {
    register_counter_vec!(
        "badshop_catalog_lookups_total",
        "Catalog lookups.",
        &[
            "user_id", // want: metrics/high-cardinality-label
            "status",
        ]
    )
    .unwrap()
}

pub fn register_searches(opts: Opts) -> CounterVec {
    CounterVec::new(
        opts,
        &[
            "session_id", // want: metrics/high-cardinality-label
        ],
    )
    .unwrap()
}

pub fn register_bounded(opts: Opts) -> CounterVec {
    CounterVec::new(
        opts,
        &[
            "tenant_id", // notwant: metrics/high-cardinality-label
            "region",
        ],
    )
    .unwrap()
}

// The metric is named after a denylisted label
pub fn register_by_url() -> CounterVec {
    register_counter_vec!(
        "url", // notwant: metrics/high-cardinality-label
        "Requests by status.",
        &["status"]
    )
    .unwrap()
}

// Cardinality is free on traces.
pub fn annotate(span: &Span, id: &str) {
    span.set_attribute(KeyValue::new("user_id", id)); // notwant: metrics/high-cardinality-label
}

// No canonical rdkafka OpenTelemetry crate exists
pub fn publish(producer: &FutureProducer, order_id: &str) {
    let record = FutureRecord::to("orders").payload(order_id); // gap: msgtrace/kafka-produce-no-inject
    let _ = producer.send(record, Duration::from_secs(0));
}

use tracing::error;

pub fn record_failure(err: &str) {
    // tracing is right there, and this line has no level and no span context.
    println!("lookup failed: {}", err); // gap: logs/unstructured-logging
    error!(error = %err, "lookup failed");
}
