#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/column-lineage-gate.json}"
OUT="${2:-results/generated/column-lineage}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.column-lineage-gate/v1" and (.symbols|length) >= 1' "$SPEC" > /dev/null

jq '
  .symbols as $S | .live_column as $live | .unused_column as $unused
  | ([ .columns[] as $c
       | {
           column: $c,
           readers: ([ $S[] | select([ .reads[]? == $c ] | any) | .symbol ] | sort),
           writers: ([ $S[] | select([ .writes[]? == $c ] | any) | .symbol ] | sort)
         }
       | .consumers = ((.readers + .writers) | unique) ]) as $lineage
  | {
      version: "patchline.column-lineage/v1",
      lineage: $lineage,
      live_column: $live,
      unused_column: $unused,
      live_consumers: ([ $lineage[] | select(.column == $live) | .consumers ] | first),
      unused_consumers: ([ $lineage[] | select(.column == $unused) | .consumers ] | first)
    }
' "$SPEC" > "$OUT/lineage.json"

{
  echo "# Column-lineage graph"
  echo
  echo "Live column $(jq -r '.live_column' "$OUT/lineage.json") consumers: $(jq -rc '.live_consumers' "$OUT/lineage.json")"
  echo "Unused column $(jq -r '.unused_column' "$OUT/lineage.json") consumers: $(jq -rc '.unused_consumers' "$OUT/lineage.json")"
} > "$OUT/lineage.md"
cp "$OUT/lineage.md" "$OUT/README.md"

echo "column-lineage worker: live_consumers=$(jq -rc '.live_consumers' "$OUT/lineage.json") unused_consumers=$(jq -rc '.unused_consumers' "$OUT/lineage.json")"
