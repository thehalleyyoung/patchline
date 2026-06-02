#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/five-minute-landing-gate.json}"; OUT="${2:-results/generated/five-minute-landing}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.five-minute-landing-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "completion rate" "make five-minute-landing-gate"; do grep -F "$phrase" docs/five-minute-landing.md README.md > /dev/null; done
bash scripts/five-minute-landing.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.five-minute-landing/v1" and .clears_threshold==true and .within_time==true and .friction_clears==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.five-minute-landing-gate-results/v1",completion_rate:$r[0].completion_rate,clears_threshold:$r[0].clears_threshold,friction_flagged:($r[0].friction_clears|not),verified:true}' > "$OUT/gate-summary.json"
echo "five-minute-landing gate passed: completion rate clears threshold within five minutes, high-friction flow flagged"
