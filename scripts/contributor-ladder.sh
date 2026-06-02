#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/contributor-ladder-gate.json}"; OUT="${2:-results/generated/contributor-ladder}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.contributor-ladder-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.criteria_defined and .mentor_assigned) ]|length) as $ok
  | {version:"patchline.contributor-ladder/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.criteria_defined and .mentor_assigned))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Contributor-ladder program with mentorship"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "contributor-ladder worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
