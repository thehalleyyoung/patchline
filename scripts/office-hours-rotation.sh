#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/office-hours-rotation-gate.json}"; OUT="${2:-results/generated/office-hours-rotation}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.office-hours-rotation-gate/v1"' "$SPEC" > /dev/null
jq '

  .slots as $S | .broken_schedule as $B
  | ([ $S[] | select((.maintainer|length)>0) ]|length) as $staffed
  | (([ $S[].maintainer ]|unique|length) == ([ $S[].maintainer ]|length)) as $noconflict
  | ([ $B[] | select((.maintainer|length)>0) ]|length) as $bstaffed
  | {version:"patchline.office-hours-rotation/v1",
     slots:($S|length), staffed:$staffed,
     full_coverage:($staffed==($S|length)),
     no_conflict:$noconflict,
     broken_full:($bstaffed==($B|length))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Office-hours triage rotation"; echo; echo "Slots $(jq -r .slots "$OUT/out.json"); staffed $(jq -r .staffed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "office-hours-rotation worker: full_coverage=$(jq -r .full_coverage "$OUT/out.json")"
