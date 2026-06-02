#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/mechanized-operational-semantics-gate.json}"; OUT="${2:-results/generated/mechanized-operational-semantics}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.mechanized-operational-semantics-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.proof_checked and ((.rule|length)>0)) ]|length) as $ok
  | {version:"patchline.mechanized-operational-semantics/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.proof_checked and ((.rule|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Mechanized operational semantics for the migration DSL"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "mechanized-operational-semantics worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
