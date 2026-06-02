#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/airgapped-distribution-gate.json}"; OUT="${2:-results/generated/airgapped-distribution}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.airgapped-distribution-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "air-gapped" "make airgapped-distribution-gate"; do grep -F "$phrase" docs/airgapped-distribution.md README.md > /dev/null; done
bash scripts/airgapped-distribution.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.airgapped-distribution/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.airgapped-distribution-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "airgapped-distribution gate passed: every gate works air-gapped, network-requiring gate rejected"
