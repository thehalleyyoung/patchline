#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/policy-as-code-layer-gate.json}"; OUT="${2:-results/generated/policy-as-code-layer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.policy-as-code-layer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "policy-as-code" "make policy-as-code-layer-gate"; do grep -F "$phrase" docs/policy-as-code-layer.md README.md > /dev/null; done
bash scripts/policy-as-code-layer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.policy-as-code-layer/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.policy-as-code-layer-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "policy-as-code-layer gate passed: every org rule mapped to a gate, unmapped rule rejected"
