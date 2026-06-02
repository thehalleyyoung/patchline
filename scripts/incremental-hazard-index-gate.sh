#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incremental-hazard-index-gate.json}"; OUT="${2:-results/generated/incremental-hazard-index}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.incremental-hazard-index-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "sub-second" "make incremental-hazard-index-gate"; do grep -F "$phrase" docs/incremental-hazard-index.md README.md > /dev/null; done
bash scripts/incremental-hazard-index.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.incremental-hazard-index/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.incremental-hazard-index-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "incremental-hazard-index gate passed: every hazard query sub-second, over-budget query rejected"
