#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/break-glass-override-gate.json}"; OUT="${2:-results/generated/break-glass-override}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.break-glass-override-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.provenance|length)>0)) ]|length) as $ok
  | {version:"patchline.break-glass-override/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.provenance|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Break-glass migration-freeze workflow"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "break-glass-override worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
