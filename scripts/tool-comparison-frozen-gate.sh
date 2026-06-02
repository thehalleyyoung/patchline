#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/tool-comparison-frozen-gate.json}"; OUT="${2:-results/generated/tool-comparison-frozen}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.tool-comparison-frozen-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "frozen" "make tool-comparison-frozen-gate"; do grep -F "$phrase" docs/tool-comparison-frozen.md README.md > /dev/null; done
bash scripts/tool-comparison-frozen.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.tool-comparison-frozen/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.tool-comparison-frozen-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "tool-comparison-frozen gate passed: every competitor measured on frozen benchmark, unmeasured competitor rejected"
