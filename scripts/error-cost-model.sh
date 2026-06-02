#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/error-cost-model-gate.json}"; OUT="${2:-results/generated/error-cost-model}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.error-cost-model-gate/v1"' "$SPEC" > /dev/null
jq '
  .severity_weight as $w
  | (.configs | map_values({raw_misses: (.missed|length), expected_cost: ([ .missed[] | $w[.] ] | add)})) as $scored
  | {
      version: "patchline.error-cost-model/v1",
      scored: $scored,
      cost_worst: ([ $scored | to_entries[] ] | max_by(.value.expected_cost) | .key),
      raw_worst: ([ $scored | to_entries[] ] | max_by(.value.raw_misses) | .key),
      ranking_flips: (([ $scored | to_entries[] ] | max_by(.value.expected_cost) | .key) != ([ $scored | to_entries[] ] | max_by(.value.raw_misses) | .key))
    }
' "$SPEC" > "$OUT/cost.json"
{ echo "# Cost-of-error model"; echo; echo "Scored: $(jq -rc '.scored' "$OUT/cost.json")"; echo "Cost-worst: $(jq -r '.cost_worst' "$OUT/cost.json"); raw-worst: $(jq -r '.raw_worst' "$OUT/cost.json"); flips: $(jq -r '.ranking_flips' "$OUT/cost.json")"; } > "$OUT/cost.md"
cp "$OUT/cost.md" "$OUT/README.md"
echo "error-cost-model worker: cost_worst=$(jq -r '.cost_worst' "$OUT/cost.json") raw_worst=$(jq -r '.raw_worst' "$OUT/cost.json") flips=$(jq -r '.ranking_flips' "$OUT/cost.json")"
