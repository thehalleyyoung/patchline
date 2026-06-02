#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/runtime-policy-monitor-gate.json}"; OUT="${2:-results/generated/runtime-policy-monitor}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.runtime-policy-monitor-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "runtime policy monitor" "make runtime-policy-monitor-gate"; do grep -F "$phrase" docs/runtime-policy-monitor.md README.md > /dev/null; done
bash scripts/runtime-policy-monitor.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.runtime-policy-monitor/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.runtime-policy-monitor-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "runtime-policy-monitor gate passed: every item scored with evidence on real self-data, unsupported item rejected"
