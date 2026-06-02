#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cutover-protocol-model-gate.json}"; OUT="${2:-results/generated/cutover-protocol-model}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.cutover-protocol-model-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.invariant_holds) ]|length) as $ok
  | {version:"patchline.cutover-protocol-model/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.invariant_holds))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Formal model of the backfill/cutover protocol"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "cutover-protocol-model worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
