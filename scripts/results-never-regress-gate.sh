#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/results-never-regress-gate.json}"; OUT="${2:-results/generated/results-never-regress}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.results-never-regress-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "never regress" "make results-never-regress-gate"; do grep -F "$phrase" docs/results-never-regress.md README.md > /dev/null; done
bash scripts/results-never-regress.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.results-never-regress/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.results-never-regress-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "results-never-regress gate passed: every release holds historical results, regressing release rejected"
