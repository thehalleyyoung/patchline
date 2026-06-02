#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/diff-minimization-gate.json}"
OUT="${2:-results/generated/diff-minimization-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.diff-minimization-gate/v1" and (.categories|length)==4' "$SPEC" > /dev/null

for phrase in "intervention diff minimization" "1-minimal" "set-cover" "make diff-minimization-gate"; do
  grep -F "$phrase" docs/diff-minimization.md README.md > /dev/null
done

bash scripts/diff-minimization.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in minimizations.json diff-minimization.json diff-minimization.md README.md; do
  test -s "$OUT/$output"
done

minf="$(jq '.minimum_findings' "$SPEC")"

jq -e --argjson minf "$minf" '
  .version == "patchline.diff-minimization/v1" and
  .findings >= $minf and
  .all_cover_universe == true and
  .all_one_minimal == true and
  .every_minimized_smaller == true and
  .reduction_ratio > 0 and
  (.total_minimized_components < .total_full_components)
' "$OUT/diff-minimization.json" > /dev/null

# Per-finding: minimized set must cover the universe and be 1-minimal.
jq -e 'all(.[]; .covers_all == true and .one_minimal == true)' "$OUT/minimizations.json" > /dev/null

jq -n --slurpfile r "$OUT/diff-minimization.json" '{
  version: "patchline.diff-minimization-gate-results/v1",
  findings: $r[0].findings,
  reduction_ratio: $r[0].reduction_ratio,
  all_one_minimal: $r[0].all_one_minimal,
  all_cover_universe: $r[0].all_cover_universe,
  verified: true
}' > "$OUT/gate-summary.json"

echo "diff minimization gate passed: reduction $(jq '.reduction_ratio' "$OUT/gate-summary.json"), one_minimal $(jq '.all_one_minimal' "$OUT/gate-summary.json")"
