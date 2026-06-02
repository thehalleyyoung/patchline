#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/negative-results-section-gate.json}"; OUT="${2:-results/generated/negative-results-section}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.negative-results-section-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.honestly_reported and ((.experiment|length)>0)) ]|length) as $ok
  | {version:"patchline.negative-results-section/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.honestly_reported and ((.experiment|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Negative-results section with experiments"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "negative-results-section worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
