#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/sustainability-plan-gate.json}"; OUT="${2:-results/generated/sustainability-plan}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.sustainability-plan-gate/v1"' "$SPEC" > /dev/null
jq '

  {version:"patchline.sustainability-plan/v1",
   ci_ok:(.ci_cost_usd <= .ci_budget_usd),
   load_ok:(.maintainer_load_hours <= .load_cap_hours),
   bus_ok:(.bus_factor >= .min_bus_factor),
   all_ok:((.ci_cost_usd <= .ci_budget_usd) and (.maintainer_load_hours <= .load_cap_hours) and (.bus_factor >= .min_bus_factor)),
   fragile_bus_ok:(.fragile_bus_factor >= .min_bus_factor)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Sustainability plan"; echo; echo "CI ok $(jq -r .ci_ok "$OUT/out.json"); bus ok $(jq -r .bus_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "sustainability-plan worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
