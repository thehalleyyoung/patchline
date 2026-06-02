#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/economic-field-study-gate.json}"; OUT="${2:-results/generated/economic-field-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.economic-field-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "confidence interval" "make economic-field-study-gate"; do grep -F "$phrase" docs/economic-field-study.md README.md > /dev/null; done
bash scripts/economic-field-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.economic-field-study/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.economic-field-study-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "economic-field-study gate passed: every item scored with evidence on real self-data, unsupported item rejected"
