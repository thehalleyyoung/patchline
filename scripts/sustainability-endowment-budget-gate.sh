#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/sustainability-endowment-budget-gate.json}"; OUT="${2:-results/generated/sustainability-endowment-budget}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.sustainability-endowment-budget-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "funded maintenance" "make sustainability-endowment-budget-gate"; do grep -F "$phrase" docs/sustainability-endowment-budget.md README.md > /dev/null; done
bash scripts/sustainability-endowment-budget.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.sustainability-endowment-budget/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.sustainability-endowment-budget-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "sustainability-endowment-budget gate passed: every item scored with evidence on real self-data, unsupported item rejected"
