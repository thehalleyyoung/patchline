#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cross-domain-transfer-gate.json}"; OUT="${2:-results/generated/cross-domain-transfer}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.cross-domain-transfer-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.transferred and (.accuracy >= .min)) ]|length) as $ok
  | {version:"patchline.cross-domain-transfer/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.transferred and (.accuracy >= .min)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Transfer to other state-transition domains"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "cross-domain-transfer worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
