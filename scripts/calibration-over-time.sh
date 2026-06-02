#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/calibration-over-time-gate.json}"; OUT="${2:-results/generated/calibration-over-time}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.calibration-over-time-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.ece <= .max_ece) ]|length) as $ok
  | {version:"patchline.calibration-over-time/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.ece <= .max_ece))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Calibration under distribution shift"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "calibration-over-time worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
