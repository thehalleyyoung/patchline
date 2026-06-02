#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/generalization-study-gate.json}"; OUT="${2:-results/generated/generalization-study}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.generalization-study-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.held_out and .disjoint) ]|length) as $ok
  | {version:"patchline.generalization-study/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.held_out and .disjoint))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Generalization study across disjoint ecosystems"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "generalization-study worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
