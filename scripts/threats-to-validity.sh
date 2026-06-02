#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/threats-to-validity-gate.json}"; OUT="${2:-results/generated/threats-to-validity}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.threats-to-validity-gate/v1"' "$SPEC" > /dev/null
jq '

  .suites as $S | .threats as $T | .unbacked_threat as $U
  | ([ $T[] | select(.backing as $b | ($S|index($b))!=null) ]|length) as $ok
  | {version:"patchline.threats-to-validity/v1",
     threats:($T|length), backed:$ok,
     all_backed:($ok==($T|length)),
     unbacked_ok:(($S|index($U.backing))!=null)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Threats-to-validity section"; echo; echo "Threats $(jq -r .threats "$OUT/out.json"); backed $(jq -r .backed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "threats-to-validity worker: all_backed=$(jq -r .all_backed "$OUT/out.json")"
