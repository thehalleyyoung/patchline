#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cross-language-hazard-atlas-gate.json}"; OUT="${2:-results/generated/cross-language-hazard-atlas}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cross-language-hazard-atlas-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "cross-language hazard atlas" "make cross-language-hazard-atlas-gate"; do grep -F "$phrase" docs/cross-language-hazard-atlas.md README.md > /dev/null; done
bash scripts/cross-language-hazard-atlas.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cross-language-hazard-atlas/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cross-language-hazard-atlas-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cross-language-hazard-atlas gate passed: every item scored with evidence on real self-data, unsupported item rejected"
