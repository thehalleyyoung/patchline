#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/column-lineage-gate.json}"
OUT="${2:-results/generated/column-lineage}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.column-lineage-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "lineage" "make column-lineage-gate"; do
  grep -F "$phrase" docs/column-lineage.md README.md > /dev/null
done

bash scripts/column-lineage.sh "$SPEC" "$OUT" > "$OUT.run.log"

# A live column lists accurate reader and writer consumers; the unused column has none.
jq -e '
  .version == "patchline.column-lineage/v1" and
  (.live_consumers | index("render_profile")) and
  (.live_consumers | index("update_email")) and
  ((.live_consumers | length) >= 2) and
  (.unused_consumers == [])
' "$OUT/lineage.json" > /dev/null

jq -n --slurpfile r "$OUT/lineage.json" '{
  version: "patchline.column-lineage-gate-results/v1",
  live_consumers: $r[0].live_consumers,
  unused_consumers: $r[0].unused_consumers,
  verified: true
}' > "$OUT/gate-summary.json"

echo "column-lineage gate passed: live column traces its consumers, unused column has none"
