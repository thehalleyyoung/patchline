#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/industry-working-group-gate.json}"; OUT="${2:-results/generated/industry-working-group}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.industry-working-group-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "working-group" "make industry-working-group-gate"; do grep -F "$phrase" docs/industry-working-group.md README.md > /dev/null; done
bash scripts/industry-working-group.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.industry-working-group/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.industry-working-group-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "industry-working-group gate passed: every item scored with evidence on real self-data, unsupported item rejected"
