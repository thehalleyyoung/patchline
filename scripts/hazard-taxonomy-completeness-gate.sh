#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/hazard-taxonomy-completeness-gate.json}"; OUT="${2:-results/generated/hazard-taxonomy-completeness}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.hazard-taxonomy-completeness-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "taxonomy completeness" "make hazard-taxonomy-completeness-gate"; do grep -F "$phrase" docs/hazard-taxonomy-completeness.md README.md > /dev/null; done
bash scripts/hazard-taxonomy-completeness.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.hazard-taxonomy-completeness/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.hazard-taxonomy-completeness-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "hazard-taxonomy-completeness gate passed: every item scored with evidence on real self-data, unsupported item rejected"
