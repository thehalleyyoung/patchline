#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/sustainability-plan-gate.json}"; OUT="${2:-results/generated/sustainability-plan}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.sustainability-plan-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "bus-factor" "make sustainability-plan-gate"; do grep -F "$phrase" docs/sustainability-plan.md README.md > /dev/null; done
bash scripts/sustainability-plan.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.sustainability-plan/v1" and .all_ok==true and .ci_ok==true and .load_ok==true and .bus_ok==true and .fragile_bus_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.sustainability-plan-gate-results/v1",all_ok:$r[0].all_ok,fragile_flagged:($r[0].fragile_bus_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "sustainability-plan gate passed: CI cost, load, and bus-factor within bounds, fragile bus-factor flagged"
