#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/multi-rater-ground-truth-gate.json}"; OUT="${2:-results/generated/multi-rater-ground-truth}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.multi-rater-ground-truth-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select((.raters>=3) and (.alpha>=0.67)) ]|length) as $ok
  | {version:"patchline.multi-rater-ground-truth/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|((.raters>=3) and (.alpha>=0.67)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Multi-rater ground-truth labeling protocol"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "multi-rater-ground-truth worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
