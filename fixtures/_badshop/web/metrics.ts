import { Counter, Histogram } from 'prom-client';

export const orders = new Counter({
  name: 'badshop_orders_total',
  help: 'Orders placed.',
  labelNames: [
    'user_id', // want: metrics/high-cardinality-label
    'status',
  ],
});

export const checkout = new Histogram({
  name: 'badshop_checkout_duration_seconds',
  help: 'Checkout latency.',
  labelNames: [
    'session_id', // want: metrics/high-cardinality-label
  ],
});

export const tenantRequests = new Counter({
  name: 'badshop_tenant_requests_total',
  help: 'Requests per tenant.',
  labelNames: [
    'tenant_id', // notwant: metrics/high-cardinality-label
    'status',
  ],
});

export function recordFailure(err: Error, status: string) {
  // The label is named 'error'; the value is the exception message.
  orders.inc({ error: err.message }); // want: metrics/unbounded-label-value
  orders.inc({ status }); // notwant: metrics/unbounded-label-value
}
