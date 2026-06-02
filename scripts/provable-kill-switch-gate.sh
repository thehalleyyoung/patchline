#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/provable-kill-switch-gate.json}"; OUT="${2:-results/generated/provable-kill-switch}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.provable-kill-switch-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "kill-switch" "make provable-kill-switch-gate"; do grep -F "$phrase" docs/provable-kill-switch.md README.md > /dev/null; done
bash scripts/provable-kill-switch.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.provable-kill-switch/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.provable-kill-switch-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "provable-kill-switch gate passed: every item scored with evidence on real self-data, unsupported item rejected"
