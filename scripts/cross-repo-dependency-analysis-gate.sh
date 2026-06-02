#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cross-repo-dependency-analysis-gate.json}"; OUT="${2:-results/generated/cross-repo-dependency-analysis}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.cross-repo-dependency-analysis-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "cross-repository" "make cross-repo-dependency-analysis-gate"; do grep -F "$phrase" docs/cross-repo-dependency-analysis.md README.md > /dev/null; done
bash scripts/cross-repo-dependency-analysis.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.cross-repo-dependency-analysis/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.cross-repo-dependency-analysis-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "cross-repo-dependency-analysis gate passed: every cross-repo hazard evidence-backed, evidence-free detection rejected"
