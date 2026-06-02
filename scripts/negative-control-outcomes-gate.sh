#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/negative-control-outcomes-gate.json}"; OUT="${2:-results/generated/negative-control-outcomes}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.negative-control-outcomes-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "negative-control outcomes" "make negative-control-outcomes-gate"; do grep -F "$phrase" docs/negative-control-outcomes.md README.md > /dev/null; done
bash scripts/negative-control-outcomes.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.negative-control-outcomes/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.negative-control-outcomes-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "negative-control-outcomes gate passed: every item scored with evidence on real self-data, unsupported item rejected"
