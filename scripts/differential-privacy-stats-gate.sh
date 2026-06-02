#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/differential-privacy-stats-gate.json}"; OUT="${2:-results/generated/differential-privacy-stats}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.differential-privacy-stats-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "differential privacy" "make differential-privacy-stats-gate"; do grep -F "$phrase" docs/differential-privacy-stats.md README.md > /dev/null; done
bash scripts/differential-privacy-stats.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.differential-privacy-stats/v1" and .noise_scale==2 and .within_bound==true and .epsilon_valid==true and .bad_epsilon_valid==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.differential-privacy-stats-gate-results/v1",noise_scale:$r[0].noise_scale,within_bound:$r[0].within_bound,bad_epsilon_rejected:($r[0].bad_epsilon_valid|not),verified:true}' > "$OUT/gate-summary.json"
echo "differential-privacy-stats gate passed: noise scale matches sensitivity/epsilon, zero-epsilon rejected"
