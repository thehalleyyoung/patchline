#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/workshop-proposal-gate.json}"; OUT="${2:-results/generated/workshop-proposal}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.workshop-proposal-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.accepted and .demo_reproducible) ]|length) as $ok
  | {version:"patchline.workshop-proposal/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.accepted and .demo_reproducible))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Conference workshop with reproducible demos"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "workshop-proposal worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
