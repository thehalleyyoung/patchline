#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/autonomy-maturity-model-gate.json}"; OUT="${2:-results/generated/autonomy-maturity-model}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.autonomy-maturity-model-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "maturity model" "make autonomy-maturity-model-gate"; do grep -F "$phrase" docs/autonomy-maturity-model.md README.md > /dev/null; done
bash scripts/autonomy-maturity-model.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.autonomy-maturity-model/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.autonomy-maturity-model-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "autonomy-maturity-model gate passed: every item scored with evidence on real self-data, unsupported item rejected"
