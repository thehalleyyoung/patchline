#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/evaluation-preregistration-gate.json}"; OUT="${2:-results/generated/evaluation-preregistration}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.evaluation-preregistration-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "pre-registration" "make evaluation-preregistration-gate"; do grep -F "$phrase" docs/evaluation-preregistration.md README.md > /dev/null; done
bash scripts/evaluation-preregistration.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.evaluation-preregistration/v1" and .matches==true and .altered_matches==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.evaluation-preregistration-gate-results/v1",matches:$r[0].matches,deviation_detected:($r[0].altered_matches|not),verified:true}' > "$OUT/gate-summary.json"
echo "evaluation-preregistration gate passed: executed protocol matches pre-registration, altered protocol detected"
