#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-gc-gate.json}"
OUT="${2:-results/generated/artifact-gc}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.artifact-gc-gate/v1" and (.cache|length) >= 1 and (.max_entries|type=="number")' "$SPEC" > /dev/null

# LRU eviction: never evict pinned entries; among unpinned, evict oldest last_used
# first, until total entries <= max_entries.
jq '
  .max_entries as $max
  | (.cache | length) as $total
  | (.cache | map(select(.pinned))) as $pinned
  | (.cache | map(select(.pinned | not)) | sort_by(.last_used)) as $unpinned_lru
  | ([ $max - ($pinned | length), 0 ] | max) as $keep_unpinned
  | ($unpinned_lru | length) as $nun
  | (if $nun > $keep_unpinned then $unpinned_lru[0:($nun - $keep_unpinned)] else [] end) as $evicted
  | ($unpinned_lru[($nun - $keep_unpinned):] // []) as $kept_unpinned
  | ($pinned + $kept_unpinned) as $survivors
  | {
      version: "patchline.artifact-gc/v1",
      max_entries: $max,
      total_before: $total,
      survivors: ($survivors | map(.key) | sort),
      evicted: ($evicted | map(.key)),
      survivor_count: ($survivors | length),
      within_budget: (($survivors | length) <= $max),
      all_pinned_survive: ($pinned | all(.[]; .key as $k | ($survivors | map(.key) | index($k)) != null)),
      evicted_in_lru_order: ($evicted | map(.last_used) == ($evicted | map(.last_used) | sort))
    }
' "$SPEC" > "$OUT/artifact-gc.json"

{
  echo "# Artifact GC (LRU, pinned-preserving)"
  echo
  echo "Budget: $(jq -r .max_entries "$OUT/artifact-gc.json") entries; before: $(jq -r .total_before "$OUT/artifact-gc.json")"
  echo
  echo "Survivors: $(jq -rc .survivors "$OUT/artifact-gc.json")"
  echo
  echo "Evicted (LRU order): $(jq -rc .evicted "$OUT/artifact-gc.json")"
} > "$OUT/artifact-gc.md"
cp "$OUT/artifact-gc.md" "$OUT/README.md"

echo "artifact-gc worker: survivors=$(jq -rc .survivors "$OUT/artifact-gc.json") evicted=$(jq -rc .evicted "$OUT/artifact-gc.json")"
