#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/limitations-gate-gate.json}"; OUT="${2:-results/generated/limitations-gate}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.limitations-gate-gate/v1"' "$SPEC" > /dev/null
jq '

  .artifacts as $A | .limitations as $L | .speculative as $S
  | ([ $L[] | select(.backing as $b | ($A|index($b))!=null) ]|length) as $ok
  | {version:"patchline.limitations-gate/v1",
     limitations:($L|length), backed:$ok,
     all_backed:($ok==($L|length)),
     speculative_ok:(($A|index($S.backing))!=null)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Backed-limitations gate"; echo; echo "Limitations $(jq -r .limitations "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "limitations-gate worker: all_backed=$(jq -r .all_backed "$OUT/out.json")"
