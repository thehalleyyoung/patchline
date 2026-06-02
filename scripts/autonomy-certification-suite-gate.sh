#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/autonomy-certification-suite-gate.json}"; OUT="${2:-results/generated/autonomy-certification-suite}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.autonomy-certification-suite-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "autonomy certification" "make autonomy-certification-suite-gate"; do grep -F "$phrase" docs/autonomy-certification-suite.md README.md > /dev/null; done
bash scripts/autonomy-certification-suite.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.autonomy-certification-suite/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.autonomy-certification-suite-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "autonomy-certification-suite gate passed: every item scored with evidence on real self-data, unsupported item rejected"
