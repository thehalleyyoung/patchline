#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/workshop-proposal-gate.json}"; OUT="${2:-results/generated/workshop-proposal}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.workshop-proposal-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "reproducible demo" "make workshop-proposal-gate"; do grep -F "$phrase" docs/workshop-proposal.md README.md > /dev/null; done
bash scripts/workshop-proposal.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.workshop-proposal/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.workshop-proposal-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "workshop-proposal gate passed: every talk has a reproducible demo, non-reproducible demo rejected"
