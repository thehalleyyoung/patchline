#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-regression-archive-gate.json}"
OUT="${2:-results/generated/intervention-regression-archive-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.intervention-regression-archive-gate/v1" and (.regression_signals|length)>=3' "$SPEC" > /dev/null

for phrase in "regression archive" "across releases" "safety" "uncertainty" "negative control" "make intervention-regression-archive-gate"; do
  grep -F "$phrase" docs/intervention-regression-archive.md README.md > /dev/null
done

bash scripts/intervention-regression-archive.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in archive/v1.0.0.jsonl archive/v1.1.0.jsonl regressions.json intervention-regression-archive.json intervention-regression-archive.md README.md; do
  test -s "$OUT/$output"
done

mini="$(jq '.minimum_interventions' "$SPEC")"

jq -e --argjson mini "$mini" '
  .version == "patchline.intervention-regression-archive/v1" and
  .interventions >= $mini and
  (.archived_releases | length) == 2 and
  .no_unexpected_regression == true and
  .clean_regressions == 0 and
  .negative_control_detected == true and
  .negative_control_regressions >= 1 and
  .stable == true
' "$OUT/intervention-regression-archive.json" > /dev/null

# Independently re-verify: the clean release diff contains zero regressed entries.
bad="$(jq '[.[] | select(.regressed == true)] | length' "$OUT/regressions.json")"
if [ "$bad" -ne 0 ]; then echo "found $bad unexpected regressions"; exit 1; fi

# And the degraded control must contain at least one.
neg="$(jq '[.[] | select(.regressed == true)] | length' "$OUT/regressions-degraded.json")"
if [ "$neg" -lt 1 ]; then echo "negative control failed to detect injected regression"; exit 1; fi

jq -n --slurpfile r "$OUT/intervention-regression-archive.json" '{
  version: "patchline.intervention-regression-archive-gate-results/v1",
  interventions: $r[0].interventions,
  archived_releases: $r[0].archived_releases,
  clean_regressions: $r[0].clean_regressions,
  negative_control_regressions: $r[0].negative_control_regressions,
  verified: true
}' > "$OUT/gate-summary.json"

echo "intervention regression archive gate passed: $(jq '.interventions' "$OUT/gate-summary.json") interventions, 0 unexpected regressions, control detected"
