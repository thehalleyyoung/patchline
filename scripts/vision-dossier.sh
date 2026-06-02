#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/vision-dossier-gate.json}"; OUT="${2:-results/generated/vision-dossier}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.vision-dossier-gate/v1"' "$SPEC" > /dev/null
jq '

  .required_pillars as $R | .pillars as $P | .incomplete_dossier as $I
  | ([ $P[].name ]) as $names
  | ([ $R[] | . as $p | ($names|index($p))!=null ]|all) as $covered
  | ([ $P[] | select((.backing|length)>0) ]|length) as $backed
  | ([ $I|map(.name) ]) as $inames
  | ([ $R[] | . as $p | ($inames[0]|index($p))!=null ]|all) as $icovered
  | {version:"patchline.vision-dossier/v1",
     pillars:($P|length),
     all_covered:($covered and ($backed==($P|length))),
     incomplete_covered:$icovered}

' "$SPEC" > "$OUT/out.json"
{ echo "# 2.0 vision dossier"; echo; echo "Pillars $(jq -r .pillars "$OUT/out.json"); all covered $(jq -r .all_covered "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "vision-dossier worker: all_covered=$(jq -r .all_covered "$OUT/out.json")"
