#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/backfill-completeness-gate.json}"
OUT="${2:-results/generated/backfill-completeness}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.backfill-completeness-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null

for phrase in "backfill" "make backfill-completeness-gate"; do
  grep -F "$phrase" docs/backfill-completeness.md README.md > /dev/null
done

bash scripts/backfill-completeness.sh "$SPEC" "$OUT" > "$OUT.run.log"

# Complete backfill certified with no uncovered rows; incomplete backfill reports id 3.
jq -e '
  .version == "patchline.backfill-completeness/v1" and
  .complete_case.complete == true and
  (.complete_case.uncovered == []) and
  .incomplete_case.complete == false and
  (.incomplete_case.uncovered == [3])
' "$OUT/backfill.json" > /dev/null

jq -n --slurpfile r "$OUT/backfill.json" '{
  version: "patchline.backfill-completeness-gate-results/v1",
  complete_certified: $r[0].complete_case.complete,
  uncovered_when_incomplete: $r[0].incomplete_case.uncovered,
  verified: true
}' > "$OUT/gate-summary.json"

echo "backfill-completeness gate passed: complete backfill certified, missing row reported"
