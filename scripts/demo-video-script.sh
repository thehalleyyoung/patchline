#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/demo-video-script-gate.json}"; OUT="${2:-results/generated/demo-video-script}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.demo-video-script-gate/v1"' "$SPEC" > /dev/null
jq '

  .beats as $B | .uncovered_beat as $U
  | ([ $B[] | select((.run|length)>0) ]|length) as $ok
  | {version:"patchline.demo-video-script/v1",
     beats:($B|length), runnable:$ok,
     all_runnable:($ok==($B|length)),
     uncovered_ok:(($U.run|length)>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Supplementary demo video script"; echo; echo "Beats $(jq -r .beats "$OUT/out.json"); runnable $(jq -r .runnable "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "demo-video-script worker: all_runnable=$(jq -r .all_runnable "$OUT/out.json")"
