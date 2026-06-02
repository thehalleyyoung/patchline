#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/integration-partner-program-gate.json}"; OUT="${2:-results/generated/integration-partner-program}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.integration-partner-program-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "certified" "make integration-partner-program-gate"; do grep -F "$phrase" docs/integration-partner-program.md README.md > /dev/null; done
bash scripts/integration-partner-program.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.integration-partner-program/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.integration-partner-program-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "integration-partner-program gate passed: every partner certified and reproducible, uncertified partner rejected"
