#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/findings-to-ticket-bridge-gate.json}"; OUT="${2:-results/generated/findings-to-ticket-bridge}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.findings-to-ticket-bridge-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.idempotent) ]|length) as $ok
  | {version:"patchline.findings-to-ticket-bridge/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.idempotent))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Findings-to-ticket bridge with idempotent sync"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "findings-to-ticket-bridge worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
