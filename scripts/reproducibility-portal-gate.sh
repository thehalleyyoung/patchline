#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reproducibility-portal-gate.json}"; OUT="${2:-results/generated/reproducibility-portal}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.reproducibility-portal-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "evidence chain" "make reproducibility-portal-gate"; do grep -F "$phrase" docs/reproducibility-portal.md README.md > /dev/null; done
bash scripts/reproducibility-portal.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.reproducibility-portal/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.reproducibility-portal-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "reproducibility-portal gate passed: every verdict has an evidence chain, chain-less verdict rejected"
