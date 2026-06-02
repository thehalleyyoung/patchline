#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/uncertainty-calibration-gate.json}"
OUT="${2:-results/generated/uncertainty-calibration-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.uncertainty-calibration-gate/v1" and (.claim|length) > 200 and (.calibrated|length) >= 1' "$SPEC" > /dev/null

for phrase in "calibration" "expected calibration error" "make uncertainty-calibration-gate"; do
  grep -F "$phrase" docs/uncertainty-calibration.md README.md > /dev/null
done

bash scripts/uncertainty-calibration.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in uncertainty-calibration.json uncertainty-calibration.md README.md; do
  test -s "$OUT/$output"
done

# Calibrated predictor within threshold (near-zero ECE); overconfident predictor exceeds
# threshold and is rejected; the two ECEs are ordered.
jq -e '
  .version == "patchline.uncertainty-calibration/v1" and
  .calibrated_ok == true and
  .overconfident_rejected == true and
  .calibrated_ece <= .threshold and
  .overconfident_ece > .threshold and
  .overconfident_ece > .calibrated_ece
' "$OUT/uncertainty-calibration.json" > /dev/null

jq -n --slurpfile r "$OUT/uncertainty-calibration.json" '{
  version: "patchline.uncertainty-calibration-gate-results/v1",
  calibrated_ece: $r[0].calibrated_ece,
  overconfident_ece: $r[0].overconfident_ece,
  verified: true
}' > "$OUT/gate-summary.json"

echo "uncertainty-calibration gate passed: calibrated within threshold, overconfident rejected"
