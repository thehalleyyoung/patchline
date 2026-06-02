#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/customer-managed-keys-gate.json}"; OUT="${2:-results/generated/customer-managed-keys}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.customer-managed-keys-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.cmk and .rotated) ]|length) as $ok
  | {version:"patchline.customer-managed-keys/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.cmk and .rotated))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Customer-managed keys with rotation"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "customer-managed-keys worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
