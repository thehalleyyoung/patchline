#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/stage-ablation-gate.json}"; OUT="${2:-results/generated/stage-ablation}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.stage-ablation-gate/v1"' "$SPEC" > /dev/null
jq '
  def r4: (.*10000|round)/10000;
  .full_accuracy as $full
  | {
      version: "patchline.stage-ablation/v1",
      full_accuracy: $full,
      marginal: (.ablated_accuracy | map_values(($full - .) | r4)),
      load_bearing: ([ .ablated_accuracy | to_entries[] | select(($full - .value) > 0) | .key ] | sort),
      redundant: ([ .ablated_accuracy | to_entries[] | select(($full - .value) == 0) | .key ] | sort)
    }
' "$SPEC" > "$OUT/ablation.json"
{ echo "# Stage ablation suite"; echo; echo "Marginal: $(jq -rc '.marginal' "$OUT/ablation.json")"; echo "Load-bearing: $(jq -rc '.load_bearing' "$OUT/ablation.json"); redundant: $(jq -rc '.redundant' "$OUT/ablation.json")"; } > "$OUT/ablation.md"
cp "$OUT/ablation.md" "$OUT/README.md"
echo "stage-ablation worker: load_bearing=$(jq -rc '.load_bearing' "$OUT/ablation.json") redundant=$(jq -rc '.redundant' "$OUT/ablation.json")"
