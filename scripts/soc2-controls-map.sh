#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/soc2-controls-map-gate.json}"; OUT="${2:-results/generated/soc2-controls-map}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.soc2-controls-map-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.automated_check|length)>0)) ]|length) as $ok
  | {version:"patchline.soc2-controls-map/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.automated_check|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# SOC2-style controls map with automated checks"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "soc2-controls-map worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
