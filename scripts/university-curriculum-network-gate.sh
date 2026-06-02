#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/university-curriculum-network-gate.json}"; OUT="${2:-results/generated/university-curriculum-network}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.university-curriculum-network-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "curriculum network" "make university-curriculum-network-gate"; do grep -F "$phrase" docs/university-curriculum-network.md README.md > /dev/null; done
bash scripts/university-curriculum-network.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.university-curriculum-network/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.university-curriculum-network-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "university-curriculum-network gate passed: every item scored with evidence on real self-data, unsupported item rejected"
