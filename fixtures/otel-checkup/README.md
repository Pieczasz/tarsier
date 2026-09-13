# otel-checkup

Bash checks for OpenTelemetry Collector config footguns (TAR-61).

```bash
./scripts/otel-checkup.sh fixtures/otel-checkup/collector-ok.yaml   # exits 0
./scripts/otel-checkup.sh fixtures/otel-checkup/collector-bad.yaml  # exits 1
```

Checks:

1. `memory_limiter` declared under `processors:`
2. `memory_limiter` is **first** in every pipeline processors list
3. `check_interval` is not `0s`
4. `batch` follows `memory_limiter` when both are present
5. Processors declared but omitted from all pipelines

License: Apache-2.0 (same as tarsier). Part of the Phase 1 flywheel (TAR-23).
