#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/acm-reproduced-badge-gate.json}"; OUT="${2:-results/generated/acm-reproduced-badge}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.acm-reproduced-badge-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.met and .signed_off) ]|length) as $ok
  | {version:"patchline.acm-reproduced-badge/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.met and .signed_off))}
' "$SPEC" > "$OUT/out.json"
{ echo "# ACM Results-Reproduced badge package"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "acm-reproduced-badge worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
