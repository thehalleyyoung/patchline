#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fraud-resistant-outcome-verification-gate.json}"; OUT="${2:-results/generated/fraud-resistant-outcome-verification}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.fraud-resistant-outcome-verification-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "fraud-resistant" "make fraud-resistant-outcome-verification-gate"; do grep -F "$phrase" docs/fraud-resistant-outcome-verification.md README.md > /dev/null; done
bash scripts/fraud-resistant-outcome-verification.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.fraud-resistant-outcome-verification/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.fraud-resistant-outcome-verification-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "fraud-resistant-outcome-verification gate passed: every item scored with evidence on real self-data, unsupported item rejected"
