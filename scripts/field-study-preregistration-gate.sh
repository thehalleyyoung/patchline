#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/field-study-preregistration-gate.json}"; OUT="${2:-results/generated/field-study-preregistration}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.field-study-preregistration-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "pre-registered" "make field-study-preregistration-gate"; do grep -F "$phrase" docs/field-study-preregistration.md README.md > /dev/null; done
bash scripts/field-study-preregistration.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.field-study-preregistration/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.field-study-preregistration-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "field-study-preregistration gate passed: every arm registered and powered, unregistered arm rejected"
