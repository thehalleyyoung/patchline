#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multimodal-finding-gate.json}"; OUT="${2:-results/generated/multimodal-finding}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.multimodal-finding-gate/v1"' "$SPEC" > /dev/null
jq '

  .findings as $F | .inconsistent as $I
  | ([ $F[] | select(.diagram_entity==.text_entity and .text_entity==.code_entity) ]|length) as $ok
  | {version:"patchline.multimodal-finding/v1",
     findings:($F|length), consistent:$ok,
     all_consistent:($ok==($F|length)),
     inconsistent_consistent:($I.diagram_entity==$I.text_entity and $I.text_entity==$I.code_entity)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Multimodal finding representation"; echo; echo "Consistent $(jq -r .consistent "$OUT/out.json")/$(jq -r .findings "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "multimodal-finding worker: all_consistent=$(jq -r .all_consistent "$OUT/out.json")"
