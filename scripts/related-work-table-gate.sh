#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/related-work-table-gate.json}"; OUT="${2:-results/generated/related-work-table}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.related-work-table-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "baseline harness" "make related-work-table-gate"; do grep -F "$phrase" docs/related-work-table.md README.md > /dev/null; done
bash scripts/related-work-table.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.related-work-table/v1" and .all_measured==true and .patchline_leads==true and .unmeasured_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.related-work-table-gate-results/v1",all_measured:$r[0].all_measured,patchline_leads:$r[0].patchline_leads,unmeasured_rejected:($r[0].unmeasured_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "related-work-table gate passed: every cell harness-measured and Patchline leads, unmeasured row rejected"
