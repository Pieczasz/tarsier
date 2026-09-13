---
name: tarsier-scan
description: Run the tarsier CLI to find observability gaps (cardinality, unstructured logs, Kafka trace propagation). Use when asked to scan a repo for observability issues, high-cardinality metrics, missing trace context, or to verify a telemetry fix. Never invent findings - only report what the CLI prints.
---

# tarsier scan

tarsier is a **deterministic** static scanner. You do not guess where logs or traces should go. You run the binary and quote its output.

## Preconditions

- Go 1.27+ and ast-grep 0.45.x on `PATH` (`brew install ast-grep` or `npm i -g @ast-grep/cli@0.45.1`).
- Build from the tarsier checkout: `make build` -> `bin/tarsier`.
  From another repo, `go install github.com/Pieczasz/tarsier/cmd/tarsier@main` if the module is reachable.

## Commands

Always scan the user's tree, not tarsier's own `_badshop`, unless they asked to demo the fixture.

```bash
tarsier scan PATH
tarsier check-cardinality PATH
tarsier --output json scan PATH
tarsier --output html scan PATH > tarsier-report.html
tarsier scan PATH --fail-on warning
```

`check-cardinality` is the free CLI slice for the cardinality-check skill (TAR-23): only high-cardinality / unbounded-label findings.

- Text: `file:line: severity: message` plus a remediation `note:`.
- JSON: the finding schema (fingerprint, rule, location, evidence.static).
- HTML: a single file, no network, for emailing.
- `--fail-on none|info|warning|error` (default `none`). Exit 1 only for **non-suppressed** findings at or above the threshold.

Baseline so an existing codebase does not fail CI on day one:

```bash
tarsier scan PATH --baseline-write tarsier-baseline.json
tarsier scan PATH --baseline tarsier-baseline.json --fail-on warning
```

## After a supposed fix

Re-run the **same** `tarsier scan` command. The finding is gone only if that fingerprint is absent from the new output. Do not claim a fix from reading the diff.

If the user wants to keep a finding, they add a reason:

```
// tarsier:ignore <rule> <reason>
```

Same shape in `#` and `--` comments.

## What you must not do

- Do not add findings that the CLI did not emit.
- Do not claim runtime confirmation (Tempo/Prometheus/Loki). This binary is static-only.
- Do not claim an Instrumentation Score. That needs OTLP.
- Do not silently apply patches. Propose a diff, then re-run the scan.

## Rules the current pack can actually fire

Quote this list if asked what tarsier covers. Anything else is backlog.

| Rule | Languages |
| -- | -- |
| `metrics/high-cardinality-label` | Go, TS, Java, Rust, Python |
| `metrics/unbounded-label-value` | Go, TS, Java, Python |
| `logs/unstructured-logging` | Go, TS |
| `msgtrace/kafka-produce-no-inject` | Go |
| `msgtrace/kafka-consume-no-extract` | Go |
