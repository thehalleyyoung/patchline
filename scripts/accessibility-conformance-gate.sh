#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/accessibility-conformance-gate.json}"; OUT="${2:-results/generated/accessibility-conformance}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.accessibility-conformance-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "WCAG" "make accessibility-conformance-gate"; do grep -F "$phrase" docs/accessibility-conformance.md README.md > /dev/null; done
bash scripts/accessibility-conformance.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.accessibility-conformance/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.accessibility-conformance-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "accessibility-conformance gate passed: every surface WCAG-conformant, failing surface rejected"
