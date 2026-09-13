import { NodeSDK } from '@opentelemetry/sdk-node';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';

// Separate file so a sibling shutdown() does not suppress the plant.
export const sdkNoStop = new NodeSDK({ // want: otel/sdk-missing-shutdown
  resource: { [ATTR_SERVICE_NAME]: 'web' },
});
