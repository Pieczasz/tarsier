# Precision gate (TAR-77)

Public high-confidence rules must earn that label. This gate applies to **new** and **changed** rules in [`Pieczasz/tarsier`](https://github.com/Pieczasz/tarsier).

## Rule

1. **Default to `medium`.** Ship new pattern rules at `confidence: medium` unless evidence says otherwise.
2. **Promote to `high` only with precision evidence:** ruletest fixtures (valid + invalid), at least one `_badshop` / corpus `notwant` FP posture note, and a short note in the rule YAML.
3. **Demote when notes admit the ceiling.** If a rule says “advisory until corpus precision is measured” or “demote if FP rate is bad”, confidence must not stay `high`.
4. **`--fail-on` / policy block** only fire on findings that are not medium-only noise paths (medium never trips `--fail-on` by design).

## Provider slices

When adding a provider family (e.g. slog bridge, TS OTel):

| Step | Done when |
| --- | --- |
| Fixtures | `fixtures/ruletests/<rule>-<lang>.yml` has valid + invalid cases |
| FP note | Rule `note:` documents the precision ceiling and stays `medium` until measured |
| README | Row added under “What it detects” |
| Evidence | Comment on TAR-77 (or child) with FP triage summary before promoting to `high` |

## This pass (TAR-77)

* Documented this gate.
* Demoted `otel/semconv-drift` (Go + TS) from `high` -> `medium` (notes already said precision unmeasured).
* Added Go **slog sprintf-message** provider (`logs/slog-sprintf-message`) at **medium** with ruletests + FP note.

No phones-home / learning telemetry in the public CLI.
