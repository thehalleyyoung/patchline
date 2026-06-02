#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/autonomous-repair-agent-gate.json}"; OUT="${2:-results/generated/autonomous-repair-agent}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.autonomous-repair-agent-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "autonomous" "make autonomous-repair-agent-gate"; do grep -F "$phrase" docs/autonomous-repair-agent.md README.md > /dev/null; done
bash scripts/autonomous-repair-agent.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.autonomous-repair-agent/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.autonomous-repair-agent-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "autonomous-repair-agent gate passed: every item scored with evidence on real self-data, unsupported item rejected"
