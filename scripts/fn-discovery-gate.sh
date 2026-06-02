#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/fn-discovery-gate.json}"
OUT="${2:-results/generated/fn-discovery-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.fn-discovery-gate/v1" and (.minimum_hazards | numbers)' "$SPEC" > /dev/null

for phrase in "False-negative discovery" "seeded" "incident analogue" "make fn-discovery-gate"; do
  grep -F "$phrase" docs/fn-discovery.md README.md > /dev/null
done

bash scripts/fn-discovery.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in fn-discovery.json fn-discovery.md hazard-recall.jsonl README.md; do
  test -s "$OUT/$output"
done

min_hazards="$(jq '.minimum_hazards' "$SPEC")"
min_specific="$(jq '.minimum_specific_detections' "$SPEC")"
min_fn="$(jq '.minimum_false_negatives' "$SPEC")"

jq -e \
  --argjson min_hazards "$min_hazards" \
  --argjson min_specific "$min_specific" \
  --argjson min_fn "$min_fn" '
  .version == "patchline.fn-discovery/v1" and
  .hazards >= $min_hazards and
  .specific_detections >= $min_specific and
  (.false_negatives | length) >= $min_fn and
  (.specific_detections + .generic_only_detections + .missed_detections) == .hazards and
  (.specific_recall >= 0 and .specific_recall <= 1) and
  .benign_control_high_severity_risks == 0 and
  .real_cross_validation.matching_kind_risks >= 1
' "$OUT/fn-discovery.json" > /dev/null

# Every hazard row must carry a valid detection tier.
jq -e -s 'all(.[]; (.detection | IN("specific","generic-only","missed")) and (.risks_on_file | type == "number"))' "$OUT/hazard-recall.jsonl" > /dev/null

jq -n --slurpfile report "$OUT/fn-discovery.json" '{
  version: "patchline.fn-discovery-gate-results/v1",
  hazards: $report[0].hazards,
  specific_recall: $report[0].specific_recall,
  false_negatives: ($report[0].false_negatives | length),
  real_cross_validation_matches: $report[0].real_cross_validation.matching_kind_risks,
  verified: true
}' > "$OUT/gate-summary.json"

echo "fn discovery gate passed: hazards $(jq '.hazards' "$OUT/gate-summary.json"), specific recall $(jq '.specific_recall' "$OUT/gate-summary.json"), surfaced false negatives $(jq '.false_negatives' "$OUT/gate-summary.json"), real xv matches $(jq '.real_cross_validation_matches' "$OUT/gate-summary.json")"
