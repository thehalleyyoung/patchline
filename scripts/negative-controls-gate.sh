#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/negative-controls-gate.json}"
OUT="${2:-results/generated/negative-controls-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.negative-controls-gate/v1" and (.min_specificity>=1.0)' "$SPEC" > /dev/null

for phrase in "Runtime-evidence negative controls" "negative control" "specificity" "make negative-controls-gate"; do
  grep -F "$phrase" docs/negative-controls.md README.md > /dev/null
done

bash scripts/negative-controls.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in paired-tests.json negative-controls.json negative-controls.md README.md; do
  test -s "$OUT/$output"
done

minf="$(jq '.minimum_findings' "$SPEC")"
minspec="$(jq '.min_specificity' "$SPEC")"
minpow="$(jq '.min_power' "$SPEC")"

jq -e --argjson minf "$minf" --argjson minspec "$minspec" --argjson minpow "$minpow" '
  .version == "patchline.negative-controls/v1" and
  .findings >= $minf and
  .specificity >= $minspec and
  .power >= $minpow and
  .false_confirmations == 0 and
  .high_severity_negative_controls > 0 and
  .high_severity_negatives_unconfirmed == true
' "$OUT/negative-controls.json" > /dev/null

# No negative-control row may ever be runtime_confirmed.
jq -e 'all(.[] | select(.arm=="negative-control"); .runtime_confirmed == false)' "$OUT/paired-tests.json" > /dev/null

jq -n --slurpfile r "$OUT/negative-controls.json" '{
  version: "patchline.negative-controls-gate-results/v1",
  findings: $r[0].findings,
  specificity: $r[0].specificity,
  power: $r[0].power,
  false_confirmations: $r[0].false_confirmations,
  high_severity_negatives_unconfirmed: $r[0].high_severity_negatives_unconfirmed,
  verified: true
}' > "$OUT/gate-summary.json"

echo "negative controls gate passed: specificity $(jq '.specificity' "$OUT/gate-summary.json"), power $(jq '.power' "$OUT/gate-summary.json")"
