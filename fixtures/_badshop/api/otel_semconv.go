package main

import (
	"go.opentelemetry.io/otel/attribute"
)

func deprecatedHTTPAttrs() {
	_ = attribute.String("http.method", "GET")       // want: otel/semconv-drift
	_ = attribute.String("http.target", "/checkout") // want: otel/semconv-drift
	_ = attribute.String("http.request.method", "GET") // notwant: otel/semconv-drift
	_ = attribute.String("url.path", "/checkout")      // notwant: otel/semconv-drift
}
