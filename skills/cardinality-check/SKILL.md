---
name: cardinality-check
description: Run tarsier check-cardinality to find high-cardinality metric labels and unbounded label values. Use when asked to check Prometheus/OTel metric cardinality, verify a label fix, or audit unbounded tags. Never invent findings — only report CLI output.
---

# cardinality-check

Free CLI slice of tarsier. Deterministic. Quote the CLI; do not guess.

## Preconditions

- `tarsier` on `PATH` (or `make build` → `./bin/tarsier` from a tarsier checkout)
- ast-grep 0.45.x on `PATH`

## Run

```bash
tarsier check-cardinality PATH
tarsier --output json check-cardinality PATH
tarsier check-cardinality PATH --fail-on warning
```

Only `metrics/high-cardinality-label` and `metrics/unbounded-label-value` are reported.

## After a supposed fix

Re-run the **same** `tarsier check-cardinality PATH` command. The finding is gone only if that fingerprint is absent. Do not claim a fix from reading the diff alone.

## Must not

- Invent findings the CLI did not emit
- Claim runtime cardinality from Prometheus/Tempo (this is static-only)
