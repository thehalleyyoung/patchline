#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/findings-to-ticket-bridge-gate.json}"; OUT="${2:-results/generated/findings-to-ticket-bridge}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.findings-to-ticket-bridge-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "idempotent" "make findings-to-ticket-bridge-gate"; do grep -F "$phrase" docs/findings-to-ticket-bridge.md README.md > /dev/null; done
bash scripts/findings-to-ticket-bridge.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.findings-to-ticket-bridge/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.findings-to-ticket-bridge-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "findings-to-ticket-bridge gate passed: every tracker syncs idempotently, duplicate-creating integration rejected"
