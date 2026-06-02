#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/causality-limits-gate.json}"
OUT="${2:-results/generated/causality-limits-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.causality-limits-gate/v1" and (.required_labels|length)==4' "$SPEC" > /dev/null

for phrase in "causality limitations" "correlation" "confounded" "make causality-limits-gate"; do
  grep -F "$phrase" docs/causality-limits.md README.md > /dev/null
done

bash scripts/causality-limits.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in scenarios.json causality-limits.json causality-limits.md README.md; do
  test -s "$OUT/$output"
done

minf="$(jq '.minimum_findings' "$SPEC")"
required="$(jq -c '.required_labels' "$SPEC")"

jq -e --argjson minf "$minf" --argjson req "$required" '
  . as $root |
  .version == "patchline.causality-limits/v1" and
  .findings >= $minf and
  .overclaims == 0 and
  .plausible_are_consistent_only == true and
  .temporal_violations_downgraded == true and
  .cross_table_unlinked == true and
  .confounded_flagged == true and
  ([$req[] | . as $l | $root.verdict_distribution | has($l)] | all)
' "$OUT/causality-limits.json" > /dev/null

# No scenario may carry an overclaiming verdict.
jq -e 'all(.[]; .overclaim == false)' "$OUT/scenarios.json" > /dev/null

jq -n --slurpfile c "$OUT/causality-limits.json" '{
  version: "patchline.causality-limits-gate-results/v1",
  scenarios: $c[0].total_scenarios,
  findings: $c[0].findings,
  overclaims: $c[0].overclaims,
  verdict_distribution: $c[0].verdict_distribution,
  verified: true
}' > "$OUT/gate-summary.json"

echo "causality limits gate passed: scenarios $(jq '.scenarios' "$OUT/gate-summary.json"), overclaims $(jq '.overclaims' "$OUT/gate-summary.json")"
