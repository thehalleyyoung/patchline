#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/global-suite-repair-synthesis-gate.json}"; OUT="${2:-results/generated/global-suite-repair-synthesis}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.global-suite-repair-synthesis-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "global-suite synthesis" "make global-suite-repair-synthesis-gate"; do grep -F "$phrase" docs/global-suite-repair-synthesis.md README.md > /dev/null; done
bash scripts/global-suite-repair-synthesis.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.global-suite-repair-synthesis/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.global-suite-repair-synthesis-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "global-suite-repair-synthesis gate passed: every item scored with evidence on real self-data, unsupported item rejected"
