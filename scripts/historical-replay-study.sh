#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/historical-replay-study-gate.json}"; OUT="${2:-results/generated/historical-replay-study}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.historical-replay-study-gate/v1"' "$SPEC" > /dev/null
jq '
  def r4: (.*10000|round)/10000;
  .migrations as $M
  | ([ $M[] | select(.outcome=="incident") ]) as $inc
  | ([ $M[] | select(.outcome=="safe") ]) as $safe
  | ([ $inc[] | select(.flagged==true) ]|length) as $tp
  | ([ $safe[] | select(.flagged==false) ]|length) as $tn
  | {
      version: "patchline.historical-replay-study/v1",
      incidents: ($inc|length),
      safe: ($safe|length),
      true_positives: $tp,
      true_negatives: $tn,
      recall: (($tp / ($inc|length)) | r4),
      specificity: (($tn / ($safe|length)) | r4),
      perfect_recall: ($tp == ($inc|length)),
      no_false_alarms: ($tn == ($safe|length))
    }
' "$SPEC" > "$OUT/replay.json"
{ echo "# Historical incident replay"; echo; echo "Recall on incidents: $(jq -r '.recall' "$OUT/replay.json"); specificity on safe: $(jq -r '.specificity' "$OUT/replay.json")"; } > "$OUT/replay.md"
cp "$OUT/replay.md" "$OUT/README.md"
echo "historical-replay-study worker: recall=$(jq -r '.recall' "$OUT/replay.json") specificity=$(jq -r '.specificity' "$OUT/replay.json")"
