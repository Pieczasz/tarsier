#!/usr/bin/env bash
# otel-checkup (TAR-61): memory_limiter + batch pipeline sanity for collector YAML.
set -euo pipefail
[[ $# -eq 1 ]] || { echo "usage: $0 <collector-config.yaml>" >&2; exit 2; }
[[ -f "$1" ]] || { echo "missing config: $1" >&2; exit 2; }
exec python3 - "$1" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
errors = []

# processors: section (between processors: and the next top-level key)
m = re.search(r"(?ms)^processors:\n(.*?)(?=^[a-z]|\Z)", text)
proc_body = m.group(1) if m else ""
declared = re.findall(r"(?m)^  ([A-Za-z0-9_/]+):", proc_body)
if "memory_limiter" not in declared:
    errors.append("memory_limiter not declared under processors:")

ml = re.search(r"(?ms)^  memory_limiter:\n((?:^    .+\n)+)", text)
if ml and re.search(r"check_interval:\s*0s?\b", ml.group(1)):
    errors.append("memory_limiter.check_interval must not be 0s")

# pipeline processor lists: "processors: [a, b]" or block lists under service.pipelines
svc = re.search(r"(?ms)^service:\n.*", text)
after = svc.group(0) if svc else ""
lists = []
for m in re.finditer(r"(?m)^\s+processors:\s*\[([^\]]*)\]", after):
    lists.append([x.strip().strip("'\"") for x in m.group(1).split(",") if x.strip()])
# block form
for m in re.finditer(r"(?ms)^\s+processors:\n((?:\s+-\s+\S+\n)+)", after):
    lists.append(re.findall(r"-\s+(\S+)", m.group(1)))

if not lists:
    errors.append("no service.pipelines processors lists found")

wired = set()
for i, procs in enumerate(lists, 1):
    wired.update(procs)
    if "memory_limiter" not in procs:
        errors.append(f"pipeline #{i}: memory_limiter missing from {procs}")
        continue
    if procs[0] != "memory_limiter":
        errors.append(f"pipeline #{i}: memory_limiter must be first, got {procs}")
    if "batch" in procs and procs.index("batch") < procs.index("memory_limiter"):
        errors.append(f"pipeline #{i}: batch must follow memory_limiter, got {procs}")

for name in declared:
    if name not in wired:
        errors.append(f"processor {name!r} declared but not wired into any pipeline")

if errors:
    print("otel-checkup FAILED:")
    for e in errors:
        print(" -", e)
    sys.exit(1)
print("otel-checkup OK")
PY
