#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/maintainer-council-gate.json}"; OUT="${2:-results/generated/maintainer-council}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.maintainer-council-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.documented) ]|length) as $ok
  | {version:"patchline.maintainer-council/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.documented))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Maintainer-council governance model"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "maintainer-council worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
