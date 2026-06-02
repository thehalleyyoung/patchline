#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/hazard-benchmark-generator-gate.json}"; OUT="${2:-results/generated/hazard-benchmark-generator}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.hazard-benchmark-generator-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "novel" "make hazard-benchmark-generator-gate"; do grep -F "$phrase" docs/hazard-benchmark-generator.md README.md > /dev/null; done
bash scripts/hazard-benchmark-generator.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.hazard-benchmark-generator/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.hazard-benchmark-generator-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "hazard-benchmark-generator gate passed: every generated hazard novel and valid, invalid or duplicate rejected"
