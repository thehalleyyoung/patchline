#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/meta-analysis-pooled-gate.json}"; OUT="${2:-results/generated/meta-analysis-pooled}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.meta-analysis-pooled-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select((.effect>0) and (.weight>0)) ]|length) as $ok
  | {version:"patchline.meta-analysis-pooled/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|((.effect>0) and (.weight>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Meta-analysis with pooled effect"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "meta-analysis-pooled worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
