#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/generalization-study-gate.json}"; OUT="${2:-results/generated/generalization-study}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.generalization-study-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "held-out" "make generalization-study-gate"; do grep -F "$phrase" docs/generalization-study.md README.md > /dev/null; done
bash scripts/generalization-study.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.generalization-study/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.generalization-study-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "generalization-study gate passed: every ecosystem held-out and disjoint, overlapping split rejected"
