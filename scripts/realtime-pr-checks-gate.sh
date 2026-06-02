#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/realtime-pr-checks-gate.json}"; OUT="${2:-results/generated/realtime-pr-checks}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.realtime-pr-checks-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "incremental" "make realtime-pr-checks-gate"; do grep -F "$phrase" docs/realtime-pr-checks.md README.md > /dev/null; done
bash scripts/realtime-pr-checks.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.realtime-pr-checks/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.realtime-pr-checks-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "realtime-pr-checks gate passed: every PR verdict sub-ten-second, over-budget check rejected"
