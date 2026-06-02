#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/calibration-over-time-gate.json}"; OUT="${2:-results/generated/calibration-over-time}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.calibration-over-time-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "calibration" "make calibration-over-time-gate"; do grep -F "$phrase" docs/calibration-over-time.md README.md > /dev/null; done
bash scripts/calibration-over-time.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.calibration-over-time/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.calibration-over-time-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "calibration-over-time gate passed: calibration holds every window, miscalibrated window rejected"
