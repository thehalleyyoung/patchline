#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/type-effect-system-gate.json}"; OUT="${2:-results/generated/type-effect-system}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.type-effect-system-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "type-and-effect" "make type-effect-system-gate"; do grep -F "$phrase" docs/type-effect-system.md README.md > /dev/null; done
bash scripts/type-effect-system.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.type-effect-system/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.type-effect-system-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "type-effect-system gate passed: every item scored with evidence on real self-data, unsupported item rejected"
