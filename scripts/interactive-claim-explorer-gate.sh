#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/interactive-claim-explorer-gate.json}"; OUT="${2:-results/generated/interactive-claim-explorer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.interactive-claim-explorer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "interactive claim explorer" "make interactive-claim-explorer-gate"; do grep -F "$phrase" docs/interactive-claim-explorer.md README.md > /dev/null; done
bash scripts/interactive-claim-explorer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.interactive-claim-explorer/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.interactive-claim-explorer-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "interactive-claim-explorer gate passed: every item scored with evidence on real self-data, unsupported item rejected"
