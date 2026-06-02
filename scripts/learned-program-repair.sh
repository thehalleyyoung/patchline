#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/learned-program-repair-gate.json}"; OUT="${2:-results/generated/learned-program-repair}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.learned-program-repair-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.proposed and .verified) ]|length) as $ok
  | {version:"patchline.learned-program-repair/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.proposed and .verified))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Learned program-repair that proposes and verifies migrations"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "learned-program-repair worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
