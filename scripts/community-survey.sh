#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/community-survey-gate.json}"; OUT="${2:-results/generated/community-survey}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.community-survey-gate/v1"' "$SPEC" > /dev/null
jq '
  .items as $I | .bad as $B
  | ([ $I[] | select(.published and .drove_roadmap) ]|length) as $ok
  | {version:"patchline.community-survey/v1",
     total:($I|length), ok:$ok,
     all_ok:($ok==($I|length)),
     bad_ok:($B|(.published and .drove_roadmap))}
' "$SPEC" > "$OUT/out.json"
{ echo "# Annual community survey driving the roadmap"; echo; echo "Checked $(jq -r .ok "$OUT/out.json")/$(jq -r .total "$OUT/out.json"); all_ok $(jq -r .all_ok "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "community-survey worker: all_ok=$(jq -r .all_ok "$OUT/out.json")"
