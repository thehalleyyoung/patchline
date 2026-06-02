#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/conference-talk-kit-gate.json}"; OUT="${2:-results/generated/conference-talk-kit}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.conference-talk-kit-gate/v1"' "$SPEC" > /dev/null
jq '

  .segments as $S | .unbacked_segment as $U
  | ([ $S[] | select((.gate|length)>0) ]|length) as $ok
  | {version:"patchline.conference-talk-kit/v1",
     segments:($S|length), backed:$ok,
     all_backed:($ok==($S|length)),
     unbacked_ok:(($U.gate|length)>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Conference-talk and tutorial kit"; echo; echo "Segments $(jq -r .segments "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "conference-talk-kit worker: all_backed=$(jq -r .all_backed "$OUT/out.json")"
