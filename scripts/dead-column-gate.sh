#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/dead-column-gate.json}"
OUT="${2:-results/generated/dead-column}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.dead-column-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "dead" "make dead-column-gate"; do
  grep -F "$phrase" docs/dead-column.md README.md > /dev/null
done

bash scripts/dead-column.sh "$SPEC" "$OUT" > "$OUT.run.log"

# Dead column has no readers and is droppable; live column has readers and is retained.
jq -e '
  .version == "patchline.dead-column/v1" and
  .dead_safe_to_drop == true and
  (.dead_readers == []) and
  .live_safe_to_drop == false and
  (.live_readers | index("login")) and
  (.live_readers | index("render_profile"))
' "$OUT/dead.json" > /dev/null

jq -n --slurpfile r "$OUT/dead.json" '{
  version: "patchline.dead-column-gate-results/v1",
  dead_droppable: $r[0].dead_safe_to_drop,
  live_readers: $r[0].live_readers,
  verified: true
}' > "$OUT/gate-summary.json"

echo "dead-column gate passed: unreferenced column droppable, still-read column retained"
