import pino from 'pino';
import '@opentelemetry/api';

const logger = pino(); // want: logs/missing-trace-correlation

export function onOrderFailed(orderId: string, err: Error) {
  console.error('order failed', orderId, err); // want: logs/unstructured-logging
  logger.error({ orderId, err }, 'order failed');
}
