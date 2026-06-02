#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/separation-logic-migrations-gate.json}"; OUT="${2:-results/generated/separation-logic-migrations}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.separation-logic-migrations-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "separation-logic" "make separation-logic-migrations-gate"; do grep -F "$phrase" docs/separation-logic-migrations.md README.md > /dev/null; done
bash scripts/separation-logic-migrations.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.separation-logic-migrations/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.separation-logic-migrations-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "separation-logic-migrations gate passed: every item scored with evidence on real self-data, unsupported item rejected"
