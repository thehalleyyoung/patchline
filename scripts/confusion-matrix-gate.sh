#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/confusion-matrix-gate.json}"; OUT="${2:-results/generated/confusion-matrix}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.confusion-matrix-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "confusion" "make confusion-matrix-gate"; do grep -F "$phrase" docs/confusion-matrix.md README.md > /dev/null; done
bash scripts/confusion-matrix.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.confusion-matrix/v1" and
  .tp == 2 and .fp == 1 and .fn == 1 and .tn == 1 and
  .precision == 0.6667 and .recall == 0.6667 and .f1 == 0.6667
' "$OUT/cm.json" > /dev/null
jq -n --slurpfile r "$OUT/cm.json" '{version:"patchline.confusion-matrix-gate-results/v1", tp:$r[0].tp, fp:$r[0].fp, fn:$r[0].fn, precision:$r[0].precision, recall:$r[0].recall, f1:$r[0].f1, verified:true}' > "$OUT/gate-summary.json"
echo "confusion-matrix gate passed: tallies and precision/recall/F1 match hand computation"
