#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-engine-matrix-gate.json}"; OUT="${2:-results/generated/multi-engine-matrix}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.multi-engine-matrix-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.semantics_defined and (.cases>0)) ]|length) as $ok
  | {version:"patchline.multi-engine-matrix/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.semantics_defined and (.cases>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Multi-database-engine semantics matrix"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "multi-engine-matrix worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
