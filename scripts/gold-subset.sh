#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/gold-subset-gate.json}"; OUT="${2:-results/generated/gold-subset}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.gold-subset-gate/v1" and (.items|length) >= 1' "$SPEC" > /dev/null
jq '
  .items as $I
  | ([ $I[] | select(.rater_a == .rater_b) ]) as $agreed
  | {
      version: "patchline.gold-subset/v1",
      total: ($I|length),
      gold: ([ $agreed[] | {id, label: .rater_a} ]),
      gold_ids: ([ $agreed[].id ] | sort),
      excluded: ([ $I[] | select(.rater_a != .rater_b) | .id ] | sort),
      agreement_rate: (($agreed|length) / ($I|length))
    }
' "$SPEC" > "$OUT/gold.json"
{ echo "# Adjudicated gold subset"; echo; echo "Gold ids: $(jq -rc '.gold_ids' "$OUT/gold.json")"; echo "Excluded: $(jq -rc '.excluded' "$OUT/gold.json"); agreement: $(jq -r '.agreement_rate' "$OUT/gold.json")"; } > "$OUT/gold.md"
cp "$OUT/gold.md" "$OUT/README.md"
echo "gold-subset worker: gold=$(jq -rc '.gold_ids' "$OUT/gold.json") agreement=$(jq -r '.agreement_rate' "$OUT/gold.json")"
