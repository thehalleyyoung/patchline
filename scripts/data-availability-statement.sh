#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/data-availability-statement-gate.json}"; OUT="${2:-results/generated/data-availability-statement}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.data-availability-statement-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.doi|length)>0) and .archived) ]|length) as $ok
  | {version:"patchline.data-availability-statement/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.doi|length)>0) and .archived))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Data-availability statement with DOI-pinned data"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "data-availability-statement worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
