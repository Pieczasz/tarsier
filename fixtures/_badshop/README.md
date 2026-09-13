# badshop - a deliberately badly-instrumented application

A small five-service shop (Go api, TypeScript web, Java worker, Rust catalog,
Python recommender) whose
instrumentation is wrong on purpose. It is the **positive control** for the
scanner: the OSS corpus can only tell us when we cry wolf, never when we miss
something, because nothing out there is labeled. Here everything is labeled.

The directory is named `_badshop` so the Go toolchain ignores it - these files
are not meant to compile, and several import packages this module does not
depend on.

## Markers

Every interesting line carries a comment naming what the scanner should do
with it. `fixtures/fixtures_test.go` reads them and asserts all four
directions, so the fixture is a complete specification: a finding on an
unmarked line fails the test just as loudly as a missed defect.

| Marker               | Meaning                                                                    | Test fails when                                         |
|----------------------|----------------------------------------------------------------------------|---------------------------------------------------------|
| `// want: <rule>`    | A real defect. The scanner must report it here.                            | Not reported - **a miss**                               |
| `// notwant: <rule>` | Looks like the defect but is correct, or lives in code we never report on. | Reported - **a false positive**                         |
| `// gap: <rule>`     | A real defect we cannot detect yet.                                        | Reported - **good news**: promote the marker to `want:` |

`notwant` lines are not filler. Each one is a false positive that actually
happened on real code, or the category of one. Across four corpus rounds the
scanner has produced 45 findings on real repositories and every one was wrong,
so these lines are the accumulated record of how.

`gap` lines are the Phase 1 worklist in executable form. When a new rule lands
and starts catching one, the test tells you to promote it. Every remaining
`gap:` below is deliberate - either a known pattern ceiling, a cut rule, or
blocked on TAR-20 (symbol resolver). Do not "finish" them by loosening
precision.

## Deliberate gaps (annotated)

| Site | Rule | Why it stays `gap:` |
|------|------|---------------------|
| `api/publish.go` build-then-produce | `msgtrace/kafka-produce-no-inject` | Message built into a variable, then sent - inline-at-produce ceiling (same as tempo/otel-demo FPs) |
| `web/publish.ts` build-then-produce | `msgtrace/kafka-produce-no-inject` | Same ceiling as Go |
| `web/publish.ts` consume | `msgtrace/kafka-consume-no-extract` | Node idiom is auto-instrumentation; source extract is the wrong shape |
| `worker/Publisher.java` build-then-send | `msgtrace/kafka-produce-no-inject` | Same ceiling as Go |
| `worker/Metrics.java` `System.out` | `logs/unstructured-logging` | Rule **cut** (8 FPs / 0 TPs on corpus) |
| `catalog/metrics.rs` Kafka produce | `msgtrace/kafka-produce-no-inject` | No canonical rdkafka OTel crate to compare against |
| `catalog/metrics.rs` `println!` | `logs/unstructured-logging` | Rule **cut** |
| `recommender/metrics.py` `print` | `logs/unstructured-logging` | Rule **cut** |

Flipped by TAR-20 (symbol resolver): `worker/Metrics.java` label-via-variable
(`LABELS` const array → `labelNames(LABELS)`), and `recommender/metrics.py`
`str(exc)` when `exc` is annotated `Exception`. OTel metrics attribute keys in
Go/TS/Python stay behind import gates.

## Layout

```
api/          Go       prometheus/promauto metrics, Kafka publish, HTTP client, SDK resource
web/          TS       prom-client metrics, OpenTelemetry metrics/spans, NodeSDK init
worker/       Java     Micrometer and simpleclient metrics, Kafka produce and consume
catalog/      Rust     register_*_vec! macros and CounterVec::new
recommender/  Python   prometheus_client and OpenTelemetry metrics
```

`api/vendor/`, `web/node_modules/` and `api/metrics_test.go` exist to prove the
scanner stays out of third-party and test code - six of the first twenty
corpus false positives lived in exactly those places.
