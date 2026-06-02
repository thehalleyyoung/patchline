#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/bug-bounty-program-gate.json}"; OUT="${2:-results/generated/bug-bounty-program}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.bug-bounty-program-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "bounty" "make bug-bounty-program-gate"; do grep -F "$phrase" docs/bug-bounty-program.md README.md > /dev/null; done
bash scripts/bug-bounty-program.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.bug-bounty-program/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.bug-bounty-program-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "bug-bounty-program gate passed: every report paid within SLA, SLA-breaching report rejected"
