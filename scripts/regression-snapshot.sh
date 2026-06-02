#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/regression-snapshot-gate.json}"; OUT="${2:-results/generated/regression-snapshot}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.regression-snapshot-gate/v1"' "$SPEC" > /dev/null
jq '

  .baseline as $B
  | ([ .current_with_new[] | select(. as $x | ($B|index($x))==null) ]) as $newhaz
  | ([ .current_no_new[] | select(. as $x | ($B|index($x))==null) ]) as $newnone
  | {version:"patchline.regression-snapshot/v1",
     baseline:($B|length),
     new_hazards:($newhaz|length),
     fails_on_new:(($newhaz|length)>0),
     passes_without_new:(($newnone|length)==0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Regression-snapshot mode"; echo; echo "New hazards $(jq -r .new_hazards "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "regression-snapshot worker: fails_on_new=$(jq -r .fails_on_new "$OUT/out.json") passes_without_new=$(jq -r .passes_without_new "$OUT/out.json")"
