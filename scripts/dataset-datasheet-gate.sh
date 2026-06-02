#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/dataset-datasheet-gate.json}"; OUT="${2:-results/generated/dataset-datasheet}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.dataset-datasheet-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "datasheet" "make dataset-datasheet-gate"; do grep -F "$phrase" docs/dataset-datasheet.md README.md > /dev/null; done
bash scripts/dataset-datasheet.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.dataset-datasheet/v1" and .complete==true and .licensed==true and .incomplete_complete==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.dataset-datasheet-gate-results/v1",complete:$r[0].complete,licensed:$r[0].licensed,incomplete_rejected:($r[0].incomplete_complete|not),verified:true}' > "$OUT/gate-summary.json"
echo "dataset-datasheet gate passed: datasheet complete with approved license, incomplete datasheet rejected"
