#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/model-card-gate.json}"; OUT="${2:-results/generated/model-card}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.model-card-gate/v1"' "$SPEC" > /dev/null
jq '

  .card as $C | .incomplete_card as $I
  | (((($C.intended_use)|length)>0) and (($C.failure_modes|length)>0) and ($C.metrics!=null)) as $complete
  | (((($I.intended_use)|length)>0) and (($I.failure_modes|length)>0) and ($I.metrics!=null)) as $icomplete
  | {version:"patchline.model-card/v1",
     failure_modes:($C.failure_modes|length),
     complete:$complete,
     incomplete_complete:$icomplete}

' "$SPEC" > "$OUT/out.json"
{ echo "# Model card"; echo; echo "Failure modes $(jq -r .failure_modes "$OUT/out.json"); complete $(jq -r .complete "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "model-card worker: complete=$(jq -r .complete "$OUT/out.json")"
