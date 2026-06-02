#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/ten-thousand-repo-funnel-gate.json}"; OUT="${2:-results/generated/ten-thousand-repo-funnel}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.ten-thousand-repo-funnel-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "retention" "make ten-thousand-repo-funnel-gate"; do grep -F "$phrase" docs/ten-thousand-repo-funnel.md README.md > /dev/null; done
bash scripts/ten-thousand-repo-funnel.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.ten-thousand-repo-funnel/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.ten-thousand-repo-funnel-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "ten-thousand-repo-funnel gate passed: every item scored with evidence on real self-data, unsupported item rejected"
