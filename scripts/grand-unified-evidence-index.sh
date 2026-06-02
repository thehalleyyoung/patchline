#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/grand-unified-evidence-index-gate.json}"; OUT="${2:-results/generated/grand-unified-evidence-index}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.grand-unified-evidence-index-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.backing_gate|length)>0)) ]|length) as $ok
  | {version:"patchline.grand-unified-evidence-index/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.backing_gate|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Grand-unified evidence index"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "grand-unified-evidence-index worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
