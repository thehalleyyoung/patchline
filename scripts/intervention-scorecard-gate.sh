#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-scorecard-gate.json}"
OUT="${2:-results/generated/intervention-scorecard-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.intervention-scorecard-gate/v1" and (.dimensions|length)==4' "$SPEC" > /dev/null

for phrase in "scorecard" "usefulness" "safety" "completeness" "uncertainty" "make intervention-scorecard-gate"; do
  grep -F "$phrase" docs/intervention-scorecard.md README.md > /dev/null
done

bash scripts/intervention-scorecard.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in scorecards.jsonl intervention-scorecard.json intervention-scorecard.md README.md; do
  test -s "$OUT/$output"
done

minc="$(jq '.minimum_interventions' "$SPEC")"

jq -e --argjson minc "$minc" '
  .version == "patchline.intervention-scorecard/v1" and
  .interventions >= $minc and
  (.dimensions | sort) == ["completeness","safety","uncertainty","usefulness"] and
  .all_in_unit_range == true and
  .all_four_present == true and
  .all_honest == true and
  .axes_separable == true and
  .overclaimed_completeness == 0 and
  .stable == true
' "$OUT/intervention-scorecard.json" > /dev/null

# Spec dimensions must match the scorecard axes exactly.
spec_dims="$(jq -c '.dimensions | sort' "$SPEC")"
out_dims="$(jq -c '.dimensions | sort' "$OUT/intervention-scorecard.json")"
if [ "$spec_dims" != "$out_dims" ]; then echo "dimension mismatch: $spec_dims vs $out_dims"; exit 1; fi

# Independently re-verify the honesty invariant: no card with open proof holes claims full
# certainty (completeness 1 AND uncertainty 0).
bad="$(jq -s 'map(select(.open_proof_holes > 0 and .scorecard.completeness >= 1 and .scorecard.uncertainty == 0)) | length' "$OUT/scorecards.jsonl")"
if [ "$bad" -ne 0 ]; then echo "found $bad overclaimed scorecards"; exit 1; fi

jq -n --slurpfile r "$OUT/intervention-scorecard.json" '{
  version: "patchline.intervention-scorecard-gate-results/v1",
  interventions: $r[0].interventions,
  means: $r[0].means,
  axes_separable: $r[0].axes_separable,
  overclaimed_completeness: $r[0].overclaimed_completeness,
  verified: true
}' > "$OUT/gate-summary.json"

echo "intervention scorecard gate passed: $(jq '.interventions' "$OUT/gate-summary.json") cards, axes separable, zero overclaims"
