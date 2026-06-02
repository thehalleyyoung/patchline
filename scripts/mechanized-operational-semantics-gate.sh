#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/mechanized-operational-semantics-gate.json}"; OUT="${2:-results/generated/mechanized-operational-semantics}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.mechanized-operational-semantics-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "operational semantics" "make mechanized-operational-semantics-gate"; do grep -F "$phrase" docs/mechanized-operational-semantics.md README.md > /dev/null; done
bash scripts/mechanized-operational-semantics.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.mechanized-operational-semantics/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.mechanized-operational-semantics-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "mechanized-operational-semantics gate passed: every reduction rule mechanized and proof-checked, unproven rule rejected"
