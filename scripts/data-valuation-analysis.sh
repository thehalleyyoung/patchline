#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/data-valuation-analysis-gate.json}"; OUT="${2:-results/generated/data-valuation-analysis}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.data-valuation-analysis-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.shapley > 0) ]|length) as $ok
  | {version:"patchline.data-valuation-analysis/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.shapley > 0))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Data-valuation of corpus examples"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "data-valuation-analysis worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
