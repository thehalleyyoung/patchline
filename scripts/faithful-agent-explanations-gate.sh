#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/faithful-agent-explanations-gate.json}"; OUT="${2:-results/generated/faithful-agent-explanations}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.faithful-agent-explanations-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "faithful explanations" "make faithful-agent-explanations-gate"; do grep -F "$phrase" docs/faithful-agent-explanations.md README.md > /dev/null; done
bash scripts/faithful-agent-explanations.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.faithful-agent-explanations/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.faithful-agent-explanations-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "faithful-agent-explanations gate passed: every item scored with evidence on real self-data, unsupported item rejected"
