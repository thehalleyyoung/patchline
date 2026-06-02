#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/risk-economics-gate.json}"
OUT="${2:-results/generated/risk-economics}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.risk-economics-gate/v1" and (.scenarios|length) >= 1' "$SPEC" > /dev/null

jq '
  {
    version: "patchline.risk-economics/v1",
    decisions: [ .scenarios[]
      | (.p_failure * .cost_failure) as $eloss
      | {
          id: .id,
          expected_loss: $eloss,
          cost_block: .cost_block,
          recommendation: (if $eloss > .cost_block then "block" else "ship" end),
          net_benefit_of_blocking: ($eloss - .cost_block)
        } ]
  }
' "$SPEC" > "$OUT/risk-economics.json"

{
  echo "# Repair-risk economics"
  echo
  jq -r '.decisions[] | "- \(.id): expected_loss=\(.expected_loss) cost_block=\(.cost_block) -> \(.recommendation)"' "$OUT/risk-economics.json"
} > "$OUT/risk-economics.md"
cp "$OUT/risk-economics.md" "$OUT/README.md"

echo "risk-economics worker: $(jq -rc '[.decisions[] | {id, recommendation}]' "$OUT/risk-economics.json")"
