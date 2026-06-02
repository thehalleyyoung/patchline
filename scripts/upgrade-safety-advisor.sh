#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/upgrade-safety-advisor-gate.json}"; OUT="${2:-results/generated/upgrade-safety-advisor}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.upgrade-safety-advisor-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(((.guided_fix|length)>0)) ]|length) as $ok
  | {version:"patchline.upgrade-safety-advisor/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(((.guided_fix|length)>0)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# In-product upgrade-safety advisor"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "upgrade-safety-advisor worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
