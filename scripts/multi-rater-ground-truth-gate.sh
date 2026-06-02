#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-rater-ground-truth-gate.json}"; OUT="${2:-results/generated/multi-rater-ground-truth}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.multi-rater-ground-truth-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "Krippendorff" "make multi-rater-ground-truth-gate"; do grep -F "$phrase" docs/multi-rater-ground-truth.md README.md > /dev/null; done
bash scripts/multi-rater-ground-truth.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.multi-rater-ground-truth/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.multi-rater-ground-truth-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "multi-rater-ground-truth gate passed: every batch multi-rated above threshold, low-agreement batch rejected"
