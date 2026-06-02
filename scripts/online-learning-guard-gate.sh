#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/online-learning-guard-gate.json}"; OUT="${2:-results/generated/online-learning-guard}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.online-learning-guard-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "online-learning guard" "make online-learning-guard-gate"; do grep -F "$phrase" docs/online-learning-guard.md README.md > /dev/null; done
bash scripts/online-learning-guard.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.online-learning-guard/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.online-learning-guard-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "online-learning-guard gate passed: every item scored with evidence on real self-data, unsupported item rejected"
