#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/related-work-table-gate.json}"; OUT="${2:-results/generated/related-work-table}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.related-work-table-gate/v1"' "$SPEC" > /dev/null
jq '

  .rows as $R | .unmeasured_row as $U
  | ([ $R[] | select(.measured) ]|length) as $m
  | ($R[] | select(.tool=="patchline")) as $p
  | ([ $R[] | select(.tool!="patchline") | $p.recall > .recall ]|all) as $leads
  | {version:"patchline.related-work-table/v1",
     rows:($R|length), measured:$m,
     all_measured:($m==($R|length)),
     patchline_leads:$leads,
     unmeasured_ok:$U.measured}

' "$SPEC" > "$OUT/out.json"
{ echo "# Related-work comparison table"; echo; echo "Rows $(jq -r .rows "$OUT/out.json"); measured $(jq -r .measured "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "related-work-table worker: all_measured=$(jq -r .all_measured "$OUT/out.json") leads=$(jq -r .patchline_leads "$OUT/out.json")"
