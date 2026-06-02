#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/proof-carrying-verdict-gate.json}"; OUT="${2:-results/generated/proof-carrying-verdict}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.proof-carrying-verdict-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.scored and ((.evidence|length)>0)) ]|length) as $ok
  | {version:"patchline.proof-carrying-verdict/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.scored and ((.evidence|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Proof-carrying verdicts"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "proof-carrying-verdict worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
