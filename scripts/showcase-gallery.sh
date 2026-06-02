#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/showcase-gallery-gate.json}"; OUT="${2:-results/generated/showcase-gallery}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.showcase-gallery-gate/v1"' "$SPEC" > /dev/null
jq '

  .entries as $E | .unbacked_entry as $U
  | ([ $E[] | select((.repo|length>0) and (.reproduce|length>0) and .reproduced) ]|length) as $ok
  | {version:"patchline.showcase-gallery/v1",
     entries:($E|length), backed:$ok,
     all_backed:($ok==($E|length)),
     unbacked_ok:(($U.reproduce|length>0) and $U.reproduced)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Showcase gallery"; echo; echo "Entries $(jq -r .entries "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "showcase-gallery worker: all_backed=$(jq -r .all_backed "$OUT/out.json")"
