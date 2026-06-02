#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/contributor-onboarding-gate.json}"; OUT="${2:-results/generated/contributor-onboarding}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.contributor-onboarding-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "one script" "make contributor-onboarding-gate"; do grep -F "$phrase" docs/contributor-onboarding.md README.md > /dev/null; done
bash scripts/contributor-onboarding.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.contributor-onboarding/v1" and .complete==true and .all_runnable==true and .incomplete_complete==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.contributor-onboarding-gate-results/v1",complete:$r[0].complete,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}' > "$OUT/gate-summary.json"
echo "contributor-onboarding gate passed: build/test/first-analysis present and runnable, incomplete plan rejected"
