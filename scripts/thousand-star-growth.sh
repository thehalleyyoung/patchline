#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/thousand-star-growth-gate.json}"; OUT="${2:-results/generated/thousand-star-growth}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.thousand-star-growth-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.measured and .reproducible) ]|length) as $ok
  | {version:"patchline.thousand-star-growth/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.measured and .reproducible))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Thousand-star growth experiment"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "thousand-star-growth worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
