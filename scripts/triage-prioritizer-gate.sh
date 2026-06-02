#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/triage-prioritizer-gate.json}"; OUT="${2:-results/generated/triage-prioritizer}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.triage-prioritizer-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "prioritize" "make triage-prioritizer-gate"; do grep -F "$phrase" docs/triage-prioritizer.md README.md > /dev/null; done
bash scripts/triage-prioritizer.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.triage-prioritizer/v1" and .duplicates_removed==1 and .ordered==true and .top=="drop-col-x"' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.triage-prioritizer-gate-results/v1",deduped:$r[0].deduped,top:$r[0].top,ordered:$r[0].ordered,verified:true}' > "$OUT/gate-summary.json"
echo "triage-prioritizer gate passed: duplicates collapsed, queue prioritized highest-impact first"
