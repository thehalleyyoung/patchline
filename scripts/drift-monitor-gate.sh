#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/drift-monitor-gate.json}"; OUT="${2:-results/generated/drift-monitor}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.drift-monitor-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "drift" "make drift-monitor-gate"; do grep -F "$phrase" docs/drift-monitor.md README.md > /dev/null; done
bash scripts/drift-monitor.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.drift-monitor/v1" and
  .same_tvd == 0 and .same_drift == false and
  (.shifted_tvd > .threshold) and .shifted_drift == true
' "$OUT/drift.json" > /dev/null
jq -n --slurpfile r "$OUT/drift.json" '{version:"patchline.drift-monitor-gate-results/v1", same_tvd:$r[0].same_tvd, shifted_tvd:$r[0].shifted_tvd, drift_flagged:$r[0].shifted_drift, verified:true}' > "$OUT/gate-summary.json"
echo "drift-monitor gate passed: identical distribution no drift, shifted distribution flagged"
