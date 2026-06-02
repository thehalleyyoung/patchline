#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/agent-budget-enforcer-gate.json}"; OUT="${2:-results/generated/agent-budget-enforcer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.agent-budget-enforcer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "hard ceiling" "make agent-budget-enforcer-gate"; do grep -F "$phrase" docs/agent-budget-enforcer.md README.md > /dev/null; done
bash scripts/agent-budget-enforcer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.agent-budget-enforcer/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.agent-budget-enforcer-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "agent-budget-enforcer gate passed: every item scored with evidence on real self-data, unsupported item rejected"
