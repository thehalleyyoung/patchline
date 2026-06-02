#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/textbook-chapter-gate.json}"; OUT="${2:-results/generated/textbook-chapter}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.textbook-chapter-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.claim_backed) ]|length) as $ok
  | {version:"patchline.textbook-chapter/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.claim_backed))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Textbook-quality chapter on migration safety"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "textbook-chapter worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
