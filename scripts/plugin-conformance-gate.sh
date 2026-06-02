#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/plugin-conformance-gate.json}"; OUT="${2:-results/generated/plugin-conformance}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.plugin-conformance-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "conformance" "make plugin-conformance-gate"; do grep -F "$phrase" docs/plugin-conformance.md README.md > /dev/null; done
bash scripts/plugin-conformance.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.plugin-conformance/v1" and .conforms==true and .conformance_rate==1 and .bad_conforms==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.plugin-conformance-gate-results/v1",conformance_rate:$r[0].conformance_rate,conforms:$r[0].conforms,bad_rejected:($r[0].bad_conforms|not),verified:true}' > "$OUT/gate-summary.json"
echo "plugin-conformance gate passed: compliant plugin conforms, incomplete plugin fails conformance"
