#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/invariant-inference-gate.json}"; OUT="${2:-results/generated/invariant-inference}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.invariant-inference-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "proof obligation" "make invariant-inference-gate"; do grep -F "$phrase" docs/invariant-inference.md README.md > /dev/null; done
bash scripts/invariant-inference.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '.version=="patchline.invariant-inference/v1" and .survivors==2 and .all_have_obligation==true and .counterexample_discarded==true' "$OUT/out.json" > /dev/null
jq -n --slurpfile r "$OUT/out.json" '{version:"patchline.invariant-inference-gate-results/v1",survivors:$r[0].survivors,counterexample_discarded:$r[0].counterexample_discarded,verified:true}' > "$OUT/gate-summary.json"
echo "invariant-inference gate passed: surviving invariants hold with obligations, counterexample discarded"
