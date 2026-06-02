#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/taint-tracker-gate.json}"
OUT="${2:-results/generated/taint-tracker}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.taint-tracker-gate/v1" and (.claim|length) > 200 and (.sources|length) >= 1 and (.sinks|length) >= 1' "$SPEC" > /dev/null

for phrase in "taint" "make taint-tracker-gate"; do
  grep -F "$phrase" docs/taint-tracker.md README.md > /dev/null
done

bash scripts/taint-tracker.sh "$SPEC" "$OUT" > "$OUT.run.log"

# The multi-hop tainted path reaches the migration-affected column; inserting a sanitizer
# node cuts that exact flow; and the clean sink (only sanitized inbound path) stays clean.
jq -e '
  .tainted_sinks as $ts | .tainted_sink as $sink |
  .version == "patchline.taint-tracker/v1" and
  .tainted_sink_reached == true and
  .tainted_sink_cut == true and
  .clean_sink_tainted == false and
  (($ts | index($sink)) != null) and
  ((.tainted_sinks_after_sanitize | index($sink)) == null)
' "$OUT/taint.json" > /dev/null

jq -n --slurpfile r "$OUT/taint.json" '{
  version: "patchline.taint-tracker-gate-results/v1",
  tainted_sinks: $r[0].tainted_sinks,
  sanitizer_cuts_flow: $r[0].tainted_sink_cut,
  clean_sink_clean: ($r[0].clean_sink_tainted | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "taint-tracker gate passed: multi-hop taint reaches sink, sanitizer cuts it, clean sink stays clean"
