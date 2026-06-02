#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/differential-semantics-gate.json}"; OUT="${2:-results/generated/differential-semantics}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.differential-semantics-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "reference semantics" "make differential-semantics-gate"; do grep -F "$phrase" docs/differential-semantics.md README.md > /dev/null; done
bash scripts/differential-semantics.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.differential-semantics/v1" and .all_agree==true and .agreement_rate==1 and .divergence_detected==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.differential-semantics-gate-results/v1",agreement_rate:$r[0].agreement_rate,divergence_detected:$r[0].divergence_detected,verified:true}' > "$OUT/gate-summary.json"
echo "differential-semantics gate passed: analyzer agrees with reference semantics, seeded divergence detected"
