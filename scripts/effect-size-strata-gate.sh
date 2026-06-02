#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/effect-size-strata-gate.json}"
OUT="${2:-results/generated/effect-size-strata-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.effect-size-strata-gate/v1" and (.minimum_slices | numbers)' "$SPEC" > /dev/null

for phrase in "Effect-size reports" "Cohen's d" "hazard-class" "standardized" "make effect-size-strata-gate"; do
  grep -F "$phrase" docs/effect-size-strata.md README.md > /dev/null
done

bash scripts/effect-size-strata.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in effect-size.json effect-size.md file-samples.jsonl risk-samples.jsonl README.md; do
  test -s "$OUT/$output"
done

min_files="$(jq '.minimum_total_files' "$SPEC")"
min_dims="$(jq '.minimum_dimensions' "$SPEC")"
min_es="$(jq '.minimum_effect_size' "$SPEC")"

jq -e --argjson min_files "$min_files" --argjson min_dims "$min_dims" --argjson min_es "$min_es" '
  .version == "patchline.effect-size-strata/v1" and
  .total_files >= $min_files and
  (.dimensions_reported | length) >= $min_dims and
  (.density_effects.ecosystem | length) >= 1 and
  (.density_effects.size | length) >= 1 and
  (.density_effects.framework | length) >= 1 and
  (.hazard_class_effects | length) >= 1 and
  (all(.density_effects.ecosystem[].cohens_d; type == "number")) and
  (all(.hazard_class_effects[].cohens_d; type == "number")) and
  ([.largest_density_effects.ecosystem.cohens_d, .largest_density_effects.size.cohens_d, .largest_density_effects.framework.cohens_d, .largest_hazard_effect.cohens_d]
    | map(if . < 0 then -. else . end) | max) >= $min_es
' "$OUT/effect-size.json" > /dev/null

jq -n --slurpfile r "$OUT/effect-size.json" '{
  version: "patchline.effect-size-strata-gate-results/v1",
  total_files: $r[0].total_files,
  total_risks: $r[0].total_risks,
  largest_density_d: ([$r[0].largest_density_effects.ecosystem.cohens_d, $r[0].largest_density_effects.size.cohens_d, $r[0].largest_density_effects.framework.cohens_d] | map(if . < 0 then -. else . end) | max),
  largest_hazard_d: ($r[0].largest_hazard_effect.cohens_d),
  verified: true
}' > "$OUT/gate-summary.json"

echo "effect-size-strata gate passed: files $(jq '.total_files' "$OUT/gate-summary.json"), risks $(jq '.total_risks' "$OUT/gate-summary.json"), largest density |d| $(jq '.largest_density_d' "$OUT/gate-summary.json"), largest hazard d $(jq '.largest_hazard_d' "$OUT/gate-summary.json")"
