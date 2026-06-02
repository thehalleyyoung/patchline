#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/self-improving-loop-gate.json}"; OUT="${2:-results/generated/self-improving-loop}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.self-improving-loop-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "unexplained" "make self-improving-loop-gate"; do grep -F "$phrase" docs/self-improving-loop.md README.md > /dev/null; done
bash scripts/self-improving-loop.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.self-improving-loop/v1" and .all_motivated==true and .unexplained==2 and .unbacked_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.self-improving-loop-gate-results/v1",unexplained:$r[0].unexplained,all_motivated:$r[0].all_motivated,unbacked_rejected:($r[0].unbacked_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "self-improving-loop gate passed: every unexplained failure yields a motivated candidate gate, unbacked proposal rejected"
