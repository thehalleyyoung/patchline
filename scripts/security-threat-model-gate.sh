#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/security-threat-model-gate.json}"; OUT="${2:-results/generated/security-threat-model}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.security-threat-model-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "threat model" "make security-threat-model-gate"; do grep -F "$phrase" docs/security-threat-model.md README.md > /dev/null; done
bash scripts/security-threat-model.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.security-threat-model/v1" and .all_mitigated==true and .coverage==1 and .unmitigated_present==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.security-threat-model-gate-results/v1",coverage:$r[0].coverage,open_risk_flagged:($r[0].unmitigated_present|not),verified:true}' > "$OUT/gate-summary.json"
echo "security-threat-model gate passed: every threat mitigated, unmitigated threat flagged as open risk"
