#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/tool-comparison-frozen-gate.json}"; OUT="${2:-results/generated/tool-comparison-frozen}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.tool-comparison-frozen-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.on_frozen_benchmark and .patchline_leads) ]|length) as $ok
  | {version:"patchline.tool-comparison-frozen/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.on_frozen_benchmark and .patchline_leads))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Comparison against published tools on a frozen benchmark"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "tool-comparison-frozen worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
