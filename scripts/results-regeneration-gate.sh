#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/results-regeneration-gate.json}"; OUT="${2:-results/generated/results-regeneration}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.results-regeneration-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "deterministically" "make results-regeneration-gate"; do grep -F "$phrase" docs/results-regeneration.md README.md > /dev/null; done
bash scripts/results-regeneration.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.results-regeneration/v1" and .all_deterministic==true and .determinism_rate==1 and .nondeterministic_matches==false' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.results-regeneration-gate-results/v1",determinism_rate:$r[0].determinism_rate,all_deterministic:$r[0].all_deterministic,nondeterministic_flagged:($r[0].nondeterministic_matches|not),verified:true}' > "$OUT/gate-summary.json"
echo "results-regeneration gate passed: figures and tables regenerate deterministically, nondeterministic artifact flagged"
