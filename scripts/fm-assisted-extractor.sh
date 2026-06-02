#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fm-assisted-extractor-gate.json}"; OUT="${2:-results/generated/fm-assisted-extractor}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.fm-assisted-extractor-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.verified_deterministic) ]|length) as $ok
  | {version:"patchline.fm-assisted-extractor/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.verified_deterministic))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Foundation-model extractor with deterministic verification"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "fm-assisted-extractor worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
