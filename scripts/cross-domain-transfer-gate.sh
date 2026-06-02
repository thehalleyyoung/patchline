#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cross-domain-transfer-gate.json}"; OUT="${2:-results/generated/cross-domain-transfer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cross-domain-transfer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "state-transition" "make cross-domain-transfer-gate"; do grep -F "$phrase" docs/cross-domain-transfer.md README.md > /dev/null; done
bash scripts/cross-domain-transfer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cross-domain-transfer/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cross-domain-transfer-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cross-domain-transfer gate passed: every domain transfers above floor, failed transfer rejected"
