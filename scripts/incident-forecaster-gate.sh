#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/incident-forecaster-gate.json}"; OUT="${2:-results/generated/incident-forecaster}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.incident-forecaster-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "scoring rule" "make incident-forecaster-gate"; do grep -F "$phrase" docs/incident-forecaster.md README.md > /dev/null; done
bash scripts/incident-forecaster.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.incident-forecaster/v1" and .beats_baseline==true and (.brier < .baseline_brier)' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.incident-forecaster-gate-results/v1",brier:$r[0].brier,baseline_brier:$r[0].baseline_brier,beats_baseline:$r[0].beats_baseline,verified:true}' > "$OUT/gate-summary.json"
echo "incident-forecaster gate passed: forecaster proper score beats uninformative baseline"
