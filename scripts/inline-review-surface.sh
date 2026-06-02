#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/inline-review-surface-gate.json}"; OUT="${2:-results/generated/inline-review-surface}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.inline-review-surface-gate/v1"' "$SPEC" > /dev/null
jq '

  .findings as $F | .broken_finding as $B
  | ([ $F[] | select((.file|length>0) and (.line>0) and (.reproduce|length>0)) ]|length) as $ok
  | {version:"patchline.inline-review-surface/v1",
     findings:($F|length), anchored:$ok,
     all_anchored:($ok==($F|length)),
     broken_anchored:(($B.file|length>0) and ($B.line>0) and ($B.reproduce|length>0))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Inline code-review surface"; echo; echo "Findings $(jq -r .findings "$OUT/out.json"); anchored $(jq -r .anchored "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "inline-review-surface worker: all_anchored=$(jq -r .all_anchored "$OUT/out.json")"
