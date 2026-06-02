#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/causal-effect-estimate-gate.json}"; OUT="${2:-results/generated/causal-effect-estimate}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.causal-effect-estimate-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  {version:"patchline.causal-effect-estimate/v1",
   adjusted_effect:(.adjusted_effect|r4),
   is_reduction:(.adjusted_effect < 0),
   adjusted_lt_naive_magnitude:(((.adjusted_effect)|if .<0 then -. else . end) < ((.unadjusted_effect)|if .<0 then -. else . end)),
   naive_biased:.naive_ignores_confounder}

' "$SPEC" > "$OUT/out.json"
{ echo "# Causal effect on incident rate"; echo; echo "Adjusted effect $(jq -r .adjusted_effect "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "causal-effect-estimate worker: is_reduction=$(jq -r .is_reduction "$OUT/out.json")"
