# godeep - compileable Go deep-tier positive control

`_badshop` is intentionally non-compiling (underscore prefix, missing
module deps). Deep-tier analysis needs `go/packages`, so this tiny module
is the proof harness for TAR-19.

Markers use `want-deep:` / `notwant-deep:` so the pattern-tier
`fixtures_test.go` ignores them. `internal/engine/golang` tests assert
the deep findings.

## Proof

`Checkout` calls `Publish` (a local wrapper around `WriteMessages`) with a
headerless `kafka.Message`. The pattern tier only matches inline
`WriteMessages(...)` and therefore misses it. The deep `msgtrace` pass
follows one hop and reports.
