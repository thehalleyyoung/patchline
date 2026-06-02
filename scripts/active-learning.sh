#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/active-learning-gate.json}"
OUT="${2:-results/generated/active-learning}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.active-learning-gate/v1" and (.examples|length) >= 1' "$SPEC" > /dev/null

jq '
  .top_k as $k
  | [ .examples[]
      | select(.labeled | not)
      # Informativeness = closeness to the 0.5 decision boundary.
      | . + {uncertainty: (0.5 - ((.confidence - 0.5) | if . < 0 then -. else . end))} ]
    as $scored
  | ($scored | sort_by(-.uncertainty)) as $ranked
  | {
      version: "patchline.active-learning/v1",
      top_k: $k,
      queue: [ $ranked[0:$k][] | {id, confidence, uncertainty} ],
      excluded_labeled: [ .examples[] | select(.labeled) | .id ],
      full_ranking: [ $ranked[] | .id ]
    }
' "$SPEC" > "$OUT/active-learning.json"

{
  echo "# Active-learning queue"
  echo
  echo "Queue (top-k): $(jq -rc '[.queue[].id]' "$OUT/active-learning.json")"
  echo
  echo "Excluded (already labeled): $(jq -rc '.excluded_labeled' "$OUT/active-learning.json")"
} > "$OUT/active-learning.md"
cp "$OUT/active-learning.md" "$OUT/README.md"

echo "active-learning worker: queue=$(jq -rc '[.queue[].id]' "$OUT/active-learning.json") excluded=$(jq -rc '.excluded_labeled' "$OUT/active-learning.json")"
