#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/realtime-pr-checks-gate.json}"; OUT="${2:-results/generated/realtime-pr-checks}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.realtime-pr-checks-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.seconds <= .budget) ]|length) as $ok
  | {version:"patchline.realtime-pr-checks/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.seconds <= .budget))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Real-time PR checks with sub-ten-second verdicts"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "realtime-pr-checks worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
