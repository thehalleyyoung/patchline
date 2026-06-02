#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/threats-to-validity-map-gate.json}"; OUT="${2:-results/generated/threats-to-validity-map}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.threats-to-validity-map-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "threats-to-validity" "make threats-to-validity-map-gate"; do grep -F "$phrase" docs/threats-to-validity-map.md README.md > /dev/null; done
bash scripts/threats-to-validity-map.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.threats-to-validity-map/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.threats-to-validity-map-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "threats-to-validity-map gate passed: every item scored with evidence on real self-data, unsupported item rejected"
