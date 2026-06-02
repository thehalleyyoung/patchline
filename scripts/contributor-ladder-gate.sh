#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/contributor-ladder-gate.json}"; OUT="${2:-results/generated/contributor-ladder}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.contributor-ladder-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "ladder" "make contributor-ladder-gate"; do grep -F "$phrase" docs/contributor-ladder.md README.md > /dev/null; done
bash scripts/contributor-ladder.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.contributor-ladder/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.contributor-ladder-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "contributor-ladder gate passed: every rung defined with a mentor, undefined rung rejected"
