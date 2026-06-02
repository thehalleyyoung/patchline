#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/limitations-gate-gate.json}"; OUT="${2:-results/generated/limitations-gate}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.limitations-gate-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "limitation" "make limitations-gate-gate"; do grep -F "$phrase" docs/limitations-gate.md README.md > /dev/null; done
bash scripts/limitations-gate.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.limitations-gate/v1" and .all_backed==true and .speculative_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.limitations-gate-gate-results/v1",backed:$r[0].backed,all_backed:$r[0].all_backed,speculative_rejected:($r[0].speculative_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "limitations-gate gate passed: every limitation demonstrably backed, speculative limitation rejected"
