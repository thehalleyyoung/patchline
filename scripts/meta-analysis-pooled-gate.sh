#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/meta-analysis-pooled-gate.json}"; OUT="${2:-results/generated/meta-analysis-pooled}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.meta-analysis-pooled-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "pooled" "make meta-analysis-pooled-gate"; do grep -F "$phrase" docs/meta-analysis-pooled.md README.md > /dev/null; done
bash scripts/meta-analysis-pooled.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.meta-analysis-pooled/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.meta-analysis-pooled-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "meta-analysis-pooled gate passed: every study contributes positive weighted effect, null-effect study rejected"
