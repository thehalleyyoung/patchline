#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/soc2-controls-map-gate.json}"; OUT="${2:-results/generated/soc2-controls-map}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.soc2-controls-map-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "control" "make soc2-controls-map-gate"; do grep -F "$phrase" docs/soc2-controls-map.md README.md > /dev/null; done
bash scripts/soc2-controls-map.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.soc2-controls-map/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.soc2-controls-map-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "soc2-controls-map gate passed: every control automated, manually-only control rejected"
