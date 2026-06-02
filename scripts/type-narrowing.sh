#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/type-narrowing-gate.json}"
OUT="${2:-results/generated/type-narrowing}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.type-narrowing-gate/v1"' "$SPEC" > /dev/null

jq '
  def classify($c):
    (if $c.to_width >= $c.from_width then "widening" else "narrowing" end) as $dir
    | {
        column: $c.column,
        direction: $dir,
        allowed: ($dir == "widening" or $c.proof == true),
        requires_proof: ($dir == "narrowing")
      };
  {
    version: "patchline.type-narrowing/v1",
    widening: classify(.widening_change),
    narrowing: classify(.narrowing_change),
    narrowing_proved: classify(.narrowing_with_proof)
  }
' "$SPEC" > "$OUT/narrowing.json"

{
  echo "# Type-narrowing safety checker"
  echo
  echo "Widening allowed: $(jq -r '.widening.allowed' "$OUT/narrowing.json")"
  echo "Narrowing (no proof) allowed: $(jq -r '.narrowing.allowed' "$OUT/narrowing.json")"
  echo "Narrowing (with proof) allowed: $(jq -r '.narrowing_proved.allowed' "$OUT/narrowing.json")"
} > "$OUT/narrowing.md"
cp "$OUT/narrowing.md" "$OUT/README.md"

echo "type-narrowing worker: widen=$(jq -r '.widening.allowed' "$OUT/narrowing.json") narrow=$(jq -r '.narrowing.allowed' "$OUT/narrowing.json") narrow_proved=$(jq -r '.narrowing_proved.allowed' "$OUT/narrowing.json")"
