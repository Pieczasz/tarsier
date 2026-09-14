import { Counter } from 'prom-client';

export const exports_total = new Counter({
  name: 'exports_total',
  help: 'exports',
  labelNames: ['email', 'status'],
});

export const ok = new Counter({ name: 'ok_total', help: 'ok', labelNames: ['status'] });

// Cardinality is free on traces: this must not be reported.
span.setAttributes({ user_id: currentUser });
