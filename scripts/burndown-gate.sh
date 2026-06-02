#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/burndown-gate.json}"
OUT="${2:-results/generated/burndown-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.burndown-gate/v1" and (.claim|length) > 200 and (.milestones|length) >= 1' "$SPEC" > /dev/null

for phrase in "roadmap burndown" "gate-backed" "make burndown-gate"; do
  grep -F "$phrase" docs/burndown.md README.md > /dev/null
done

bash scripts/burndown.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in burndown.json burndown.md README.md; do
  test -s "$OUT/$output"
done

# All four real deliverables have existing gates and count complete; the phantom
# deliverable points at a missing gate and stays outstanding.
jq -e '
  .version == "patchline.burndown/v1" and
  .total_deliverables == 4 and
  .complete_deliverables == 4 and
  .remaining_deliverables == 0 and
  .percent_complete == 100 and
  (.milestones | all(.[]; (.deliverables | all(.[]; .complete)))) and
  .negative_control.complete == false
' "$OUT/burndown.json" > /dev/null

jq -n --slurpfile r "$OUT/burndown.json" '{
  version: "patchline.burndown-gate-results/v1",
  percent_complete: $r[0].percent_complete,
  phantom_outstanding: ($r[0].negative_control.complete | not),
  verified: true
}' > "$OUT/gate-summary.json"

echo "burndown gate passed: only existing-gate deliverables count complete, phantom stays outstanding"
