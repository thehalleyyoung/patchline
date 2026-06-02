#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/localized-quickstarts-gate.json}"; OUT="${2:-results/generated/localized-quickstarts}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.localized-quickstarts-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "localized" "make localized-quickstarts-gate"; do grep -F "$phrase" docs/localized-quickstarts.md README.md > /dev/null; done
bash scripts/localized-quickstarts.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.localized-quickstarts/v1" and .full_parity==true and .incomplete_parity==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.localized-quickstarts-gate-results/v1",full_parity:$r[0].full_parity,incomplete_flagged:($r[0].incomplete_parity|not),verified:true}' > "$OUT/gate-summary.json"
echo "localized-quickstarts gate passed: every locale has full step parity, incomplete locale flagged"
