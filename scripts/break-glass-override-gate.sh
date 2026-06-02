#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/break-glass-override-gate.json}"; OUT="${2:-results/generated/break-glass-override}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.break-glass-override-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "break-glass" "make break-glass-override-gate"; do grep -F "$phrase" docs/break-glass-override.md README.md > /dev/null; done
bash scripts/break-glass-override.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.break-glass-override/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.break-glass-override-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "break-glass-override gate passed: every override provenance-logged, unlogged override rejected"
