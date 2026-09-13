import { metrics, trace } from '@opentelemetry/api';

const meter = metrics.getMeter('badshop');
const checkouts = meter.createCounter('badshop_checkouts_total');

export function recordCheckout(userId: string, status: string) {
  checkouts.add(1, {
    user_id: userId, // want: metrics/high-cardinality-label
    status,
  });
}

export function annotateSpan(userId: string) {
  trace.getActiveSpan()?.setAttributes({
    user_id: userId, // notwant: metrics/high-cardinality-label
  });
}

export const audit = {
  record(event: { user_id: string }) {
    return event;
  },
};

audit.record({
  user_id: 'alice', // notwant: metrics/high-cardinality-label
});
