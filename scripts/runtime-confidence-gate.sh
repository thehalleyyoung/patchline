#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/runtime-confidence-gate.json}"
OUT="${2:-results/generated/runtime-confidence-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.runtime-confidence-gate/v1" and (.runtime_threshold | numbers)' "$SPEC" > /dev/null

for phrase in "Runtime-evidence confidence scoring" "static" "runtime" "quadrant" "make runtime-confidence-gate"; do
  grep -F "$phrase" docs/runtime-confidence.md README.md > /dev/null
done

bash scripts/runtime-confidence.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in runtime-confidence.json runtime-confidence.md scored-findings.json runtime-evidence.jsonl README.md; do
  test -s "$OUT/$output"
done

min_findings="$(jq '.minimum_findings' "$SPEC")"
min_quad="$(jq '.minimum_quadrants_populated' "$SPEC")"
min_div="$(jq '.minimum_divergence' "$SPEC")"

jq -e --argjson min_findings "$min_findings" --argjson min_quad "$min_quad" --argjson min_div "$min_div" '
  .version == "patchline.runtime-confidence/v1" and
  .total_findings >= $min_findings and
  .quadrants_populated >= $min_quad and
  .divergence >= $min_div and
  # Both confirmed and static-only must exist: runtime confirms some, leaves others unconfirmed.
  .quadrants.confirmed >= 1 and
  .quadrants["static-only"] >= 1 and
  (.quadrants.confirmed + .quadrants["static-only"] + .quadrants["runtime-only"] + .quadrants.quiet) == .total_findings and
  (.confidence_min >= 0 and .confidence_max <= 1)
' "$OUT/runtime-confidence.json" > /dev/null

# Runtime evidence must be independent of severity: telemetry/impact derive from table hash,
# so at least one finding has high static risk WITHOUT observed impact (static-only) AND the
# runtime axis must take both values across findings.
jq -e '
  ([.[] | .runtime_axis] | unique | length) == 2 and
  (any(.[]; .static_axis >= 0.7 and .runtime_axis == 0)) and
  (any(.[]; .runtime_axis == 1))
' "$OUT/scored-findings.json" > /dev/null

jq -n --slurpfile r "$OUT/runtime-confidence.json" '{
  version: "patchline.runtime-confidence-gate-results/v1",
  total_findings: $r[0].total_findings,
  quadrants_populated: $r[0].quadrants_populated,
  divergence: $r[0].divergence,
  verified: true
}' > "$OUT/gate-summary.json"

echo "runtime confidence gate passed: findings $(jq '.total_findings' "$OUT/gate-summary.json"), quadrants $(jq '.quadrants_populated' "$OUT/gate-summary.json"), divergence $(jq '.divergence' "$OUT/gate-summary.json")"
