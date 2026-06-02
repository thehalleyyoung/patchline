#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/artifact-gc-gate.json}"
OUT="${2:-results/generated/artifact-gc-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.artifact-gc-gate/v1" and (.claim|length) > 200 and (.cache|length) >= 1' "$SPEC" > /dev/null

for phrase in "LRU" "pinned" "make artifact-gc-gate"; do
  grep -F "$phrase" docs/artifact-gc.md README.md > /dev/null
done

bash scripts/artifact-gc.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in artifact-gc.json artifact-gc.md README.md; do
  test -s "$OUT/$output"
done

# Survivors within budget; all pinned survive (even the oldest entry "a");
# eviction strictly LRU order; the specific oldest unpinned entries (b,c) evicted.
jq -e '
  .version == "patchline.artifact-gc/v1" and
  .within_budget == true and
  .all_pinned_survive == true and
  .evicted_in_lru_order == true and
  (.survivors | index("a") != null) and
  (.evicted == ["b","c"])
' "$OUT/artifact-gc.json" > /dev/null

jq -n --slurpfile r "$OUT/artifact-gc.json" '{
  version: "patchline.artifact-gc-gate-results/v1",
  survivors: $r[0].survivors,
  evicted: $r[0].evicted,
  verified: true
}' > "$OUT/gate-summary.json"

echo "artifact-gc gate passed: within budget, pinned preserved, LRU eviction order (evicted b,c)"
