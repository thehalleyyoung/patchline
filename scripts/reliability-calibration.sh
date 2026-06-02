#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reliability-calibration-gate.json}"; OUT="${2:-results/generated/reliability-calibration}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.reliability-calibration-gate/v1"' "$SPEC" > /dev/null
jq '
  def r4: (.*10000|round)/10000;
  def absf: if . < 0 then -. else . end;
  def ece($bins): ($bins | map(.count) | add) as $N
    | ([ $bins[] | (.count/$N) * ((.confidence - .accuracy)|absf) ] | add) | r4;
  .ece_threshold as $t
  | {
      version: "patchline.reliability-calibration/v1",
      threshold: $t,
      well_ece: ece(.well_calibrated),
      well_ok: (ece(.well_calibrated) <= $t),
      over_ece: ece(.overconfident),
      over_ok: (ece(.overconfident) <= $t)
    }
' "$SPEC" > "$OUT/calib.json"
{ echo "# Reliability calibration study"; echo; echo "Well-calibrated ECE: $(jq -r '.well_ece' "$OUT/calib.json") (<= $(jq -r '.threshold' "$OUT/calib.json"): $(jq -r '.well_ok' "$OUT/calib.json"))"; echo "Overconfident ECE: $(jq -r '.over_ece' "$OUT/calib.json") ok=$(jq -r '.over_ok' "$OUT/calib.json")"; } > "$OUT/calib.md"
cp "$OUT/calib.md" "$OUT/README.md"
echo "reliability-calibration worker: well_ece=$(jq -r '.well_ece' "$OUT/calib.json") over_ece=$(jq -r '.over_ece' "$OUT/calib.json")"
