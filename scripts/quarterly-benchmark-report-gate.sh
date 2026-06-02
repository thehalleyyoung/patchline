#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/quarterly-benchmark-report-gate.json}"; OUT="${2:-results/generated/quarterly-benchmark-report}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.quarterly-benchmark-report-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "leaderboard" "make quarterly-benchmark-report-gate"; do grep -F "$phrase" docs/quarterly-benchmark-report.md README.md > /dev/null; done
bash scripts/quarterly-benchmark-report.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.quarterly-benchmark-report/v1" and .ordered==true and .non_regressing==true and .regression_nonreg==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.quarterly-benchmark-report-gate-results/v1",non_regressing:$r[0].non_regressing,regression_flagged:($r[0].regression_nonreg|not),verified:true}' > "$OUT/gate-summary.json"
echo "quarterly-benchmark-report gate passed: series ordered and non-regressing, injected regression flagged"
