import { NodeSDK } from '@opentelemetry/sdk-node';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';

export const sdk = new NodeSDK({}); // want: otel/sdk-missing-resource-attrs

export const sdkOK = new NodeSDK({ // notwant: otel/sdk-missing-resource-attrs
  resource: { [ATTR_SERVICE_NAME]: 'web' },
});

// Shutdown exists in-file so sdk-missing-shutdown is asserted in sdk-nostop.ts.
void sdk.shutdown();
