#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/negative-results-section-gate.json}"; OUT="${2:-results/generated/negative-results-section}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.negative-results-section-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "negative-results" "make negative-results-section-gate"; do grep -F "$phrase" docs/negative-results-section.md README.md > /dev/null; done
bash scripts/negative-results-section.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.negative-results-section/v1" and .all_ok==true and .bad_ok==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.negative-results-section-gate-results/v1",ok:$r[0].ok,all_ok:$r[0].all_ok,bad_rejected:($r[0].bad_ok|not),verified:true}' > "$OUT/gate-summary.json"
echo "negative-results-section gate passed: every negative result experiment-backed, unsupported claim rejected"
