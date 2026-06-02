#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/partner-hazard-sdk-gate.json}"; OUT="${2:-results/generated/partner-hazard-sdk}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.partner-hazard-sdk-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "conformance tests" "make partner-hazard-sdk-gate"; do grep -F "$phrase" docs/partner-hazard-sdk.md README.md > /dev/null; done
bash scripts/partner-hazard-sdk.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.partner-hazard-sdk/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.partner-hazard-sdk-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "partner-hazard-sdk gate passed: every item scored with evidence on real self-data, unsupported item rejected"
