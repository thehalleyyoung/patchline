#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/dataset-datasheet-gate.json}"; OUT="${2:-results/generated/dataset-datasheet}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.dataset-datasheet-gate/v1"' "$SPEC" > /dev/null
jq '

  .required_sections as $R | .datasheet as $D | .approved_licenses as $AL | .incomplete_datasheet as $I
  | ([ $R[] | . as $s | ($D|has($s)) ]|all) as $complete
  | (($AL|index($D.licensing))!=null) as $licensed
  | ([ $R[] | . as $s | ($I|has($s)) ]|all) as $icomplete
  | {version:"patchline.dataset-datasheet/v1",
     sections:($R|length), complete:$complete, licensed:$licensed,
     incomplete_complete:$icomplete}

' "$SPEC" > "$OUT/out.json"
{ echo "# Dataset datasheet"; echo; echo "Complete $(jq -r .complete "$OUT/out.json"); licensed $(jq -r .licensed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "dataset-datasheet worker: complete=$(jq -r .complete "$OUT/out.json") licensed=$(jq -r .licensed "$OUT/out.json")"
