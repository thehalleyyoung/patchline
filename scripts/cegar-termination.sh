#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/cegar-termination-gate.json}"; OUT="${2:-results/generated/cegar-termination}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.cegar-termination-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.terminated and (.iterations <= .bound)) ]|length) as $ok
  | {version:"patchline.cegar-termination/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.terminated and (.iterations <= .bound)))}
' "$SPEC" > "$OUT/out.json"
{ echo "# CEGAR loop with a termination proof"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "cegar-termination worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
