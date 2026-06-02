#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-tenant-isolation-gate.json}"; OUT="${2:-results/generated/multi-tenant-isolation}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.multi-tenant-isolation-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "isolation" "make multi-tenant-isolation-gate"; do grep -F "$phrase" docs/multi-tenant-isolation.md README.md > /dev/null; done
bash scripts/multi-tenant-isolation.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.multi-tenant-isolation/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.multi-tenant-isolation-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "multi-tenant-isolation gate passed: every tenant isolated with no leak, leaking tenant rejected"
