#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/backfill-formal-synthesis-gate.json}"; OUT="${2:-results/generated/backfill-formal-synthesis}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.backfill-formal-synthesis-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.synthesized and .establishes_invariant) ]|length) as $ok
  | {version:"patchline.backfill-formal-synthesis/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.synthesized and .establishes_invariant))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Formal synthesis of backfills from invariants"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "backfill-formal-synthesis worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
