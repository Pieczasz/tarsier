# tarsier

Static scanner for observability gaps in application code: high-cardinality metric labels, unstructured logs, and Kafka produce/consume without trace context. Runs on a checkout. Does not need your telemetry backend.

## Install

Prerequisites: Go 1.27+ and [ast-grep](https://ast-grep.github.io/) 0.45.x on `PATH`.

```bash
# macOS
brew install ast-grep
# or: npm i -g @ast-grep/cli@0.45.1

go install github.com/Pieczasz/tarsier/cmd/tarsier@v0.1.0
```

Or from source:

```bash
git clone https://github.com/Pieczasz/tarsier.git
cd tarsier
make build
./bin/tarsier scan fixtures/_badshop
```

## Quick start

```bash
tarsier scan path/to/repo
tarsier --output json scan path/to/repo
tarsier --output html scan path/to/repo > tarsier-report.html
tarsier scan --engine=all path/to/go/module   # pattern + Go deep tier
```

`--engine pattern|go|all` (default `pattern`). Medium-confidence findings never trip `--fail-on`.

CI gate without a policy file:

```bash
tarsier scan --fail-on warning .
```

Adopt on an existing repo without failing the first build:

```bash
tarsier scan . --baseline-write tarsier-baseline.json
tarsier scan . --baseline tarsier-baseline.json --fail-on warning
```

Quiet one finding in source (reason required):

```go
// tarsier:ignore metrics/high-cardinality-label tenant label is bounded here
```

Example workflow: [`examples/github-actions/scan.yml`](examples/github-actions/scan.yml).

Agents: copy [`skills/tarsier-scan/SKILL.md`](skills/tarsier-scan/SKILL.md) or
[`skills/cardinality-check/SKILL.md`](skills/cardinality-check/SKILL.md) into
your Cursor/Claude skills. Skills only run this CLI; they do not invent findings.

## What it detects

| Rule | Languages | Notes |
| --- | --- | --- |
| `metrics/high-cardinality-label` | Go, TS, Java, Rust, Python | Literal unbounded label names at declaration |
| `metrics/unbounded-label-value` | Go, TS, Java, Python | `err.Error()` / uuid-shaped values at the call site |
| `logs/unstructured-logging` | Go, TS | `log.Printf`/`console.*` where slog/zap/pino/winston is imported |
| `msgtrace/kafka-produce-no-inject` | Go, TS, Java | Headerless produce (medium on TS/Java when inject is invisible) |
| `msgtrace/kafka-consume-no-extract` | Go, Java | Read without `.Headers`/`Extract` |
| `httpctx/outbound-call-without-context` | Go | `http.Get`/`Post`/`Head`/`PostForm` |
| `otel/sdk-missing-resource-attrs` | Go, TS | Provider/SDK without `service.name` |
| `otel/sdk-missing-shutdown` | Go, TS | Provider without Shutdown/forceFlush (medium) |
| `errors/swallowed-on-critical-path` | Go, Java | `_ = err` / empty `catch` (medium) |
| `traces/error-path-not-recorded-on-span` | Go | return err after `.Start(` without RecordError/SetStatus (medium) |
| `logs/missing-trace-correlation` | Go, TS | slog/pino alongside OTel without a bridge (medium) |

## Demo fixture

[`fixtures/_badshop`](fixtures/_badshop) is a deliberately broken multi-language app. Every interesting line is marked `want:` / `notwant:` / `gap:`.

```bash
make build
./bin/tarsier scan fixtures/_badshop
./bin/tarsier --output html scan fixtures/_badshop > /tmp/tarsier-badshop.html
./bin/tarsier scan --fail-on warning fixtures/_badshop; echo exit:$?
```

See [`DEMO.md`](DEMO.md) for a five-minute walkthrough.

## License

Apache-2.0. See [LICENSE](LICENSE).
