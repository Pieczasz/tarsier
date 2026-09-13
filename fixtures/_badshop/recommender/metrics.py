"""Recommendation service instrumentation, wrong on purpose."""

import logging

from opentelemetry import metrics, trace
from prometheus_client import Counter, Histogram

logger = logging.getLogger(__name__)

lookups = Counter(
    "badshop_recommender_lookups_total",
    "Recommendation lookups.",
    [
        "user_id",  # want: metrics/high-cardinality-label
        "status",
    ],
)

latency = Histogram(
    "badshop_recommender_seconds",
    "Recommendation latency.",
    labelnames=[
        "email",  # want: metrics/high-cardinality-label
    ],
)

per_tenant = Counter(
    "badshop_recommender_tenant_total",
    "Per tenant.",
    [
        "tenant_id",  # notwant: metrics/high-cardinality-label
        "region",
    ],
)

meter = metrics.get_meter("badshop.recommender")
served = meter.create_counter("badshop_recommender_served_total")


def record_served(user_id: str, status: str) -> None:
    served.add(
        1,
        {
            "session_id": user_id,  # want: metrics/high-cardinality-label
            "status": status,
        },
    )


def annotate(user_id: str) -> None:
    # Cardinality is free on traces.
    trace.get_current_span().set_attribute("user_id", user_id)  # notwant: metrics/high-cardinality-label


def audit(user_id: str) -> None:
    # Not an instrument: the dict is the only argument.
    audit_sink.record({"user_id": user_id})  # notwant: metrics/high-cardinality-label


def notify(order_id: str) -> None:
    print(f"notified customer for order {order_id}")  # gap: logs/unstructured-logging


def record_failure(exc: Exception, status: str, request) -> None:
    # Exception-typed str() is unbounded; enum-shaped str() stays a miss
    # (prefect's str(message.kind) FP). TAR-20 resolves the typed case only.
    lookups.labels(str(exc)).inc()  # want: metrics/unbounded-label-value

    # Unbounded by construction, so these are provable without type information.
    lookups.labels(request.path).inc()  # want: metrics/unbounded-label-value
    lookups.labels(traceback.format_exc()).inc()  # want: metrics/unbounded-label-value

    lookups.labels(status).inc()  # notwant: metrics/unbounded-label-value
