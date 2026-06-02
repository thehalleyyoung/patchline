#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/extensibility-proof-gate.json}"; OUT="${2:-results/generated/extensibility-proof}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.extensibility-proof-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.added_cost_lines <= .budget) ]|length) as $ok
  | {version:"patchline.extensibility-proof/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.added_cost_lines <= .budget))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Successor-tool extensibility proof"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "extensibility-proof worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
