#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/eval-matrix-gate.json}"
OUT="${2:-results/generated/eval-matrix}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.eval-matrix-gate/v1" and (.criteria|length) >= 1' "$SPEC" > /dev/null

score_one() {
  # $1 = JSON criterion {name, artifacts}; checks each artifact path exists on disk.
  local crit="$1" name present total
  name="$(jq -r '.name' <<<"$crit")"
  total="$(jq -r '.artifacts|length' <<<"$crit")"
  present=0
  while IFS= read -r p; do
    [ -z "$p" ] && continue
    if [ -e "$p" ]; then present=$((present+1)); fi
  done < <(jq -r '.artifacts[]' <<<"$crit")
  jq -n --arg n "$name" --argjson present "$present" --argjson total "$total" \
    '{name:$n, artifacts_declared:$total, artifacts_present:$present, supported:($present > 0)}'
}

matrix="[]"
while IFS= read -r crit; do
  row="$(score_one "$crit")"
  matrix="$(jq --argjson r "$row" '. + [$r]' <<<"$matrix")"
done < <(jq -c '.criteria[]' "$SPEC")

neg="$(score_one "$(jq -c '.negative_control' "$SPEC")")"

jq -n --argjson matrix "$matrix" --argjson neg "$neg" '{
  version: "patchline.eval-matrix/v1",
  matrix: $matrix,
  all_supported: ($matrix | all(.[]; .supported)),
  unsupported: ($matrix | map(select(.supported|not) | .name)),
  negative_control: $neg
}' > "$OUT/eval-matrix.json"

{
  echo "# Best-paper evaluation matrix"
  echo
  echo "All criteria supported: $(jq -r '.all_supported' "$OUT/eval-matrix.json")"
  echo
  echo "Negative-control criterion supported: $(jq -r '.negative_control.supported' "$OUT/eval-matrix.json")"
} > "$OUT/eval-matrix.md"
cp "$OUT/eval-matrix.md" "$OUT/README.md"

echo "eval-matrix worker: all_supported=$(jq -r '.all_supported' "$OUT/eval-matrix.json") neg_supported=$(jq -r '.negative_control.supported' "$OUT/eval-matrix.json")"
