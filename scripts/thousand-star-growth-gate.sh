#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/thousand-star-growth-gate.json}"; OUT="${2:-results/generated/thousand-star-growth}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.thousand-star-growth-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "funnel" "make thousand-star-growth-gate"; do grep -F "$phrase" docs/thousand-star-growth.md README.md > /dev/null; done
bash scripts/thousand-star-growth.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.thousand-star-growth/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.thousand-star-growth-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "thousand-star-growth gate passed: every intervention measured and reproducible, unmeasured intervention rejected"
