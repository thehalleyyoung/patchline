#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reliability-calibration-gate.json}"; OUT="${2:-results/generated/reliability-calibration}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.reliability-calibration-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "calibration" "make reliability-calibration-gate"; do grep -F "$phrase" docs/reliability-calibration.md README.md > /dev/null; done
bash scripts/reliability-calibration.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.reliability-calibration/v1" and
  .well_ok == true and (.well_ece <= .threshold) and
  .over_ok == false and (.over_ece > .threshold)
' "$OUT/calib.json" > /dev/null
jq -n --slurpfile r "$OUT/calib.json" '{version:"patchline.reliability-calibration-gate-results/v1", well_ece:$r[0].well_ece, over_ece:$r[0].over_ece, verified:true}' > "$OUT/gate-summary.json"
echo "reliability-calibration gate passed: well-calibrated under threshold, overconfident exceeds it"
