#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/sustainability-endowment-gate.json}"; OUT="${2:-results/generated/sustainability-endowment}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.sustainability-endowment-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.funded) ]|length) as $ok
  | {version:"patchline.sustainability-endowment/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.funded))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Sustainability endowment plan"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "sustainability-endowment worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
