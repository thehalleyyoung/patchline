#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/hardware-cost-model-gate.json}"; OUT="${2:-results/generated/hardware-cost-model}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.hardware-cost-model-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.throughput>0 and .dollars>0 and .per_dollar>0) ]|length) as $ok
  | {version:"patchline.hardware-cost-model/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.throughput>0 and .dollars>0 and .per_dollar>0))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Hardware-cost throughput-per-dollar model"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "hardware-cost-model worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
