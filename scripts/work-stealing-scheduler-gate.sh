#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/work-stealing-scheduler-gate.json}"; OUT="${2:-results/generated/work-stealing-scheduler}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.work-stealing-scheduler-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "work-stealing" "make work-stealing-scheduler-gate"; do grep -F "$phrase" docs/work-stealing-scheduler.md README.md > /dev/null; done
bash scripts/work-stealing-scheduler.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.work-stealing-scheduler/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.work-stealing-scheduler-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "work-stealing-scheduler gate passed: every task runs exactly once, lost or duplicated task rejected"
