#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/soundness-boundary-gate.json}"; OUT="${2:-results/generated/soundness-boundary}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.soundness-boundary-gate/v1"' "$SPEC" > /dev/null
jq '

  .classes as $C | .ungated_guarantee as $U
  | ([ $C[] | select((.level|length)>0) ]|length) as $leveled
  | ([ $C[] | select(.level=="guaranteed") ]) as $g
  | ([ $g[] | select((.backing_gates|length)>0) ]|length) as $gbacked
  | {version:"patchline.soundness-boundary/v1",
     classes:($C|length), all_leveled:($leveled==($C|length)),
     guaranteed:($g|length), guaranteed_backed:$gbacked,
     all_guaranteed_backed:($gbacked==($g|length)),
     ungated_backed:(($U.backing_gates|length)>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Soundness-boundary specification"; echo; echo "Classes $(jq -r .classes "$OUT/out.json"); guaranteed $(jq -r .guaranteed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "soundness-boundary worker: all_guaranteed_backed=$(jq -r .all_guaranteed_backed "$OUT/out.json")"
