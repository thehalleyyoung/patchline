#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/dead-column-gate.json}"
OUT="${2:-results/generated/dead-column}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.dead-column-gate/v1"' "$SPEC" > /dev/null

jq '
  def readers_of($col): [ .reads[] | select(.column == $col) | .symbol ] | unique;
  (readers_of(.dead_column)) as $dead_r
  | (readers_of(.live_column)) as $live_r
  | {
      version: "patchline.dead-column/v1",
      dead_column: .dead_column,
      dead_readers: $dead_r,
      dead_safe_to_drop: (($dead_r | length) == 0),
      live_column: .live_column,
      live_readers: $live_r,
      live_safe_to_drop: (($live_r | length) == 0)
    }
' "$SPEC" > "$OUT/dead.json"

{
  echo "# Dead-column detector"
  echo
  echo "Dead column $(jq -r '.dead_column' "$OUT/dead.json") safe to drop: $(jq -r '.dead_safe_to_drop' "$OUT/dead.json")"
  echo "Live column $(jq -r '.live_column' "$OUT/dead.json") readers: $(jq -rc '.live_readers' "$OUT/dead.json")"
} > "$OUT/dead.md"
cp "$OUT/dead.md" "$OUT/README.md"

echo "dead-column worker: dead_safe=$(jq -r '.dead_safe_to_drop' "$OUT/dead.json") live_safe=$(jq -r '.live_safe_to_drop' "$OUT/dead.json")"
