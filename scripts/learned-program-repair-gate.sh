#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/learned-program-repair-gate.json}"; OUT="${2:-results/generated/learned-program-repair}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.learned-program-repair-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "verifies" "make learned-program-repair-gate"; do grep -F "$phrase" docs/learned-program-repair.md README.md > /dev/null; done
bash scripts/learned-program-repair.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.learned-program-repair/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.learned-program-repair-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "learned-program-repair gate passed: every repair proposal verified, unverified proposal rejected"
