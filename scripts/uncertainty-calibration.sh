#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/uncertainty-calibration-gate.json}"
OUT="${2:-results/generated/uncertainty-calibration}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.uncertainty-calibration-gate/v1" and (.calibrated|length) >= 1 and (.threshold|type=="number")' "$SPEC" > /dev/null

# Expected calibration error: bin by stated confidence, compare mean confidence to
# observed accuracy in each bin, weight by bin size.
jq '
  def ece($preds):
    ($preds | length) as $n
    | ($preds | group_by((.confidence * 10 | round) ))
    | map({
        bin_size: length,
        mean_conf: (map(.confidence) | add / length),
        accuracy: (map(if .correct then 1 else 0 end) | add / length)
      })
    | (map(.bin_size * (((.mean_conf - .accuracy) | if . < 0 then -. else . end))) | add) / $n;
  {
    version: "patchline.uncertainty-calibration/v1",
    threshold: .threshold,
    calibrated_ece: (ece(.calibrated)),
    overconfident_ece: (ece(.overconfident))
  }
  | . + {
      calibrated_ok: (.calibrated_ece <= .threshold),
      overconfident_rejected: (.overconfident_ece > .threshold)
    }
' "$SPEC" > "$OUT/uncertainty-calibration.json"

{
  echo "# Uncertainty calibration"
  echo
  echo "Threshold (max ECE): $(jq -r .threshold "$OUT/uncertainty-calibration.json")"
  echo
  echo "- Calibrated predictor ECE: $(jq -r .calibrated_ece "$OUT/uncertainty-calibration.json") (ok=$(jq -r .calibrated_ok "$OUT/uncertainty-calibration.json"))"
  echo "- Overconfident predictor ECE: $(jq -r .overconfident_ece "$OUT/uncertainty-calibration.json") (rejected=$(jq -r .overconfident_rejected "$OUT/uncertainty-calibration.json"))"
} > "$OUT/uncertainty-calibration.md"
cp "$OUT/uncertainty-calibration.md" "$OUT/README.md"

echo "uncertainty-calibration worker: calibrated_ece=$(jq -r .calibrated_ece "$OUT/uncertainty-calibration.json") overconfident_ece=$(jq -r .overconfident_ece "$OUT/uncertainty-calibration.json")"
