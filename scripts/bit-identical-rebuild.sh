#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/bit-identical-rebuild-gate.json}"; OUT="${2:-results/generated/bit-identical-rebuild}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.bit-identical-rebuild-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.rebuilt and .identical_hash) ]|length) as $ok
  | {version:"patchline.bit-identical-rebuild/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.rebuilt and .identical_hash))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Bit-identical rebuild from frozen snapshot"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "bit-identical-rebuild worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
