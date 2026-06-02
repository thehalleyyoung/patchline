#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/perturbation-robustness-gate.json}"; OUT="${2:-results/generated/perturbation-robustness}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.perturbation-robustness-gate/v1"' "$SPEC" > /dev/null
jq '
  def r4: (.*10000|round)/10000;
  .base_verdict as $b | .preserving as $P
  | ([ $P[] | select(.verdict == $b) ] | length) as $stable
  | {
      version: "patchline.perturbation-robustness/v1",
      base_verdict: $b,
      total_preserving: ($P|length),
      stable_count: $stable,
      stability_rate: (($stable / ($P|length)) | r4),
      fully_stable: ($stable == ($P|length)),
      semantic_flips: (.semantic_change.verdict != $b)
    }
' "$SPEC" > "$OUT/robust.json"
{ echo "# Perturbation robustness suite"; echo; echo "Stability: $(jq -r '.stable_count' "$OUT/robust.json")/$(jq -r '.total_preserving' "$OUT/robust.json") (rate $(jq -r '.stability_rate' "$OUT/robust.json"))"; echo "Semantic change flips verdict: $(jq -r '.semantic_flips' "$OUT/robust.json")"; } > "$OUT/robust.md"
cp "$OUT/robust.md" "$OUT/README.md"
echo "perturbation-robustness worker: fully_stable=$(jq -r '.fully_stable' "$OUT/robust.json") semantic_flips=$(jq -r '.semantic_flips' "$OUT/robust.json")"
