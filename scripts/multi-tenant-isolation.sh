#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-tenant-isolation-gate.json}"; OUT="${2:-results/generated/multi-tenant-isolation}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.multi-tenant-isolation-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.isolated and (.leaked==false)) ]|length) as $ok
  | {version:"patchline.multi-tenant-isolation/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.isolated and (.leaked==false)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Multi-tenant isolation with no-cross-tenant-leak"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "multi-tenant-isolation worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
