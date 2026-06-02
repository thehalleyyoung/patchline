#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-site-rct-gate.json}"; OUT="${2:-results/generated/multi-site-rct}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.multi-site-rct-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "randomized controlled trial" "make multi-site-rct-gate"; do grep -F "$phrase" docs/multi-site-rct.md README.md > /dev/null; done
bash scripts/multi-site-rct.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.multi-site-rct/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.multi-site-rct-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "multi-site-rct gate passed: every item scored with evidence on real self-data, unsupported item rejected"
