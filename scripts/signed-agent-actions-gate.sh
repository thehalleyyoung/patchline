#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/signed-agent-actions-gate.json}"; OUT="${2:-results/generated/signed-agent-actions}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.signed-agent-actions-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "signed agent actions" "make signed-agent-actions-gate"; do grep -F "$phrase" docs/signed-agent-actions.md README.md > /dev/null; done
bash scripts/signed-agent-actions.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.signed-agent-actions/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.signed-agent-actions-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "signed-agent-actions gate passed: every item scored with evidence on real self-data, unsupported item rejected"
