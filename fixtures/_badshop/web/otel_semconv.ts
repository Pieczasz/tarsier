import { Span } from '@opentelemetry/api';

export function drift(span: Span) {
  span.setAttribute('http.method', 'GET'); // want: otel/semconv-drift
  span.setAttribute('http.target', '/checkout'); // want: otel/semconv-drift
  span.setAttribute('http.request.method', 'GET'); // notwant: otel/semconv-drift
  span.setAttribute('url.path', '/checkout'); // notwant: otel/semconv-drift
}
