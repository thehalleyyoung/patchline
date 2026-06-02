#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/hazard-benchmark-generator-gate.json}"; OUT="${2:-results/generated/hazard-benchmark-generator}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.hazard-benchmark-generator-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.novel and .valid) ]|length) as $ok
  | {version:"patchline.hazard-benchmark-generator/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.novel and .valid))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Automated generator of novel hazards"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "hazard-benchmark-generator worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
