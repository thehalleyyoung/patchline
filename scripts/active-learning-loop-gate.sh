#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/active-learning-loop-gate.json}"; OUT="${2:-results/generated/active-learning-loop}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.active-learning-loop-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "informative" "make active-learning-loop-gate"; do grep -F "$phrase" docs/active-learning-loop.md README.md > /dev/null; done
bash scripts/active-learning-loop.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.active-learning-loop/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.active-learning-loop-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "active-learning-loop gate passed: every queried case informative, low-information query rejected"
