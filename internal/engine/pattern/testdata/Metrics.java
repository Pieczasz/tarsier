import io.micrometer.core.instrument.Counter;

class Metrics {
  void register(MeterRegistry reg, String id) {
    Counter.builder("orders_total").tag("session_id", id).register(reg);
  }

  void fine(MeterRegistry reg, String status) {
    Counter.builder("ok_total").tag("status", status).register(reg);
  }

  void spanIsFine(Span span, String id) {
    span.setAttribute("user_id", id);
  }
}
