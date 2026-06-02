#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/polyglot-orm-frontend-gate.json}"; OUT="${2:-results/generated/polyglot-orm-frontend}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.polyglot-orm-frontend-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.shared_core and ((.extractor|length)>0)) ]|length) as $ok
  | {version:"patchline.polyglot-orm-frontend/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.shared_core and ((.extractor|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Polyglot ORM front-end with shared core"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "polyglot-orm-frontend worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
