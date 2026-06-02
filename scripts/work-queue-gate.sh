#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/work-queue-gate.json}"; OUT="${2:-results/generated/work-queue}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.work-queue-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "queue" "make work-queue-gate"; do grep -F "$phrase" docs/work-queue.md README.md > /dev/null; done
bash scripts/work-queue.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.work-queue/v1" and
  .deterministic == true and .complete == true and .disjoint == true and
  .corrupt_has_overlap == true
' "$OUT/queue.json" > /dev/null
jq -n --slurpfile r "$OUT/queue.json" '{version:"patchline.work-queue-gate-results/v1", deterministic:$r[0].deterministic, complete:$r[0].complete, disjoint:$r[0].disjoint, corrupt_detected:$r[0].corrupt_has_overlap, verified:true}' > "$OUT/gate-summary.json"
echo "work-queue gate passed: deterministic complete disjoint partition, duplicate detected"
