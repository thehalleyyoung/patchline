#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/corpus-bias-correction-gate.json}"; OUT="${2:-results/generated/corpus-bias-correction}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.corpus-bias-correction-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "post-stratification" "make corpus-bias-correction-gate"; do grep -F "$phrase" docs/corpus-bias-correction.md README.md > /dev/null; done
bash scripts/corpus-bias-correction.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.corpus-bias-correction/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.corpus-bias-correction-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "corpus-bias-correction gate passed: every item scored with evidence on real self-data, unsupported item rejected"
