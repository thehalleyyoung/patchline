#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/self-serve-onboarding-gate.json}"; OUT="${2:-results/generated/self-serve-onboarding}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.self-serve-onboarding-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "retention" "make self-serve-onboarding-gate"; do grep -F "$phrase" docs/self-serve-onboarding.md README.md > /dev/null; done
bash scripts/self-serve-onboarding.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.self-serve-onboarding/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.self-serve-onboarding-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "self-serve-onboarding gate passed: every cohort clears activation and retention, churned cohort flagged"
