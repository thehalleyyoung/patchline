#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/autonomous-fleet-orchestrator-gate.json}"; OUT="${2:-results/generated/autonomous-fleet-orchestrator}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.autonomous-fleet-orchestrator-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "autonomous fleet" "make autonomous-fleet-orchestrator-gate"; do grep -F "$phrase" docs/autonomous-fleet-orchestrator.md README.md > /dev/null; done
bash scripts/autonomous-fleet-orchestrator.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.autonomous-fleet-orchestrator/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.autonomous-fleet-orchestrator-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "autonomous-fleet-orchestrator gate passed: every item scored with evidence on real self-data, unsupported item rejected"
