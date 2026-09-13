package shop.worker;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.opentelemetry.api.trace.Span;
import org.slf4j.Logger;

public class Metrics {

  public void registerOrders(MeterRegistry registry, String orderId) {
    Counter.builder("badshop_orders_total")
        .tag("order_id", orderId) // want: metrics/high-cardinality-label
        .register(registry);
  }

  public void registerLegacy() {
    io.prometheus.client.Counter.build()
        .name("badshop_legacy_requests_total")
        .labelNames("session_id") // want: metrics/high-cardinality-label
        .register();
  }

  public void registerBounded(MeterRegistry registry, String status) {
    Counter.builder("badshop_ok_total")
        .tag("status", status) // notwant: metrics/high-cardinality-label
        .register(registry);
  }

  // Cardinality is free on traces. Flagging span attributes would be wrong.
  public void annotate(Span span, String userId) {
    span.setAttribute("user_id", userId); // notwant: metrics/high-cardinality-label
  }

  // The blind spot found by reading apache/pulsar: the labels are in a
  // variable, so a literal denylist cannot see them.
  private static final String[] LABELS = {
    "user_id", // want: metrics/high-cardinality-label
  };

  // The tag is named "error", which passes any name-based denylist, while the
  // value is a whole exception message.
  public void recordFailure(MeterRegistry registry, Exception e, String status) {
    Counter.builder("badshop_orders_failed_total")
        .tag("error", e.getMessage()) // want: metrics/unbounded-label-value
        .register(registry);
    Counter.builder("badshop_orders_failed_total")
        .tag("status", status) // notwant: metrics/unbounded-label-value
        .register(registry);
  }

  // slf4j is imported in this file, so this bypasses it entirely.
  public void report(String orderId) {
    System.out.println("order failed " + orderId); // gap: logs/unstructured-logging
  }

  public void registerIndirect() {
    io.prometheus.client.Counter.build()
        .name("badshop_indirect_total")
        .labelNames(LABELS)
        .register();
  }
}
