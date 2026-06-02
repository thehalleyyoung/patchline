#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reviewer-reproduction-guide-gate.json}"; OUT="${2:-results/generated/reviewer-reproduction-guide}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.reviewer-reproduction-guide-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "in minutes" "make reviewer-reproduction-guide-gate"; do grep -F "$phrase" docs/reviewer-reproduction-guide.md README.md > /dev/null; done
bash scripts/reviewer-reproduction-guide.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.reviewer-reproduction-guide/v1" and .within_step_budget==true and .within_time_budget==true and .reaches_headline==true and .bloated_within_budget==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.reviewer-reproduction-guide-gate-results/v1",within_step_budget:$r[0].within_step_budget,within_time_budget:$r[0].within_time_budget,bloated_rejected:($r[0].bloated_within_budget|not),verified:true}' > "$OUT/gate-summary.json"
echo "reviewer-reproduction-guide gate passed: guide within page and time budget reaching headline, bloated guide rejected"
