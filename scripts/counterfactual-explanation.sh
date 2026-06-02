#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/counterfactual-explanation-gate.json}"; OUT="${2:-results/generated/counterfactual-explanation}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.counterfactual-explanation-gate/v1"' "$SPEC" > /dev/null
jq '

  {version:"patchline.counterfactual-explanation/v1",
   base_verdict:.base_verdict,
   edits:(.counterfactual_edits|length),
   flips:(.flipped_verdict != .base_verdict),
   minimal:.minimal,
   nonflip_flips:(.non_flipping.flipped_verdict != .base_verdict)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Counterfactual explanation"; echo; echo "Edits $(jq -r .edits "$OUT/out.json"); flips $(jq -r .flips "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "counterfactual-explanation worker: flips=$(jq -r .flips "$OUT/out.json") minimal=$(jq -r .minimal "$OUT/out.json")"
