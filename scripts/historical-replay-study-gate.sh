#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/historical-replay-study-gate.json}"; OUT="${2:-results/generated/historical-replay-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.historical-replay-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "ground truth" "make historical-replay-study-gate"; do grep -F "$phrase" docs/historical-replay-study.md README.md > /dev/null; done
bash scripts/historical-replay-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.historical-replay-study/v1" and
  .perfect_recall == true and .no_false_alarms == true and
  .recall == 1 and .specificity == 1
' "$OUT/replay.json" > /dev/null
jq -n --slurpfile r "$OUT/replay.json" '{version:"patchline.historical-replay-study-gate-results/v1", recall:$r[0].recall, specificity:$r[0].specificity, verified:true}' > "$OUT/gate-summary.json"
echo "historical-replay-study gate passed: perfect recall on incidents, no false alarms on safe migrations"
