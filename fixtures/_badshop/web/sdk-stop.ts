import { NodeSDK } from '@opentelemetry/sdk-node';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';

export async function stopOK() {
  const local = new NodeSDK({ // notwant: otel/sdk-missing-shutdown
    resource: { [ATTR_SERVICE_NAME]: 'web' },
  });
  await local.shutdown();
  return local;
}
