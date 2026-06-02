#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cegar-termination-gate.json}"; OUT="${2:-results/generated/cegar-termination}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cegar-termination-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "termination" "make cegar-termination-gate"; do grep -F "$phrase" docs/cegar-termination.md README.md > /dev/null; done
bash scripts/cegar-termination.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cegar-termination/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cegar-termination-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cegar-termination gate passed: every CEGAR run terminates within bound, non-terminating run rejected"
