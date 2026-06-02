#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/a11y-i18n-output-gate.json}"; OUT="${2:-results/generated/a11y-i18n-output}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.a11y-i18n-output-gate/v1"' "$SPEC" > /dev/null
jq '

  .messages as $M | .catalog as $C | .bad_message as $B
  | ([ $M[] | select((.text_marker|length>0) and (.catalog_key|length>0)) ]|length) as $ok
  | ([ $M[] | select(.catalog_key as $k | ($C|index($k))!=null) ]|length) as $covered
  | {version:"patchline.a11y-i18n-output/v1",
     messages:($M|length), accessible:$ok, covered:$covered,
     all_accessible:($ok==($M|length)),
     all_localizable:($covered==($M|length)),
     bad_accessible:(($B.text_marker|length>0) and ($B.catalog_key|length>0))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Accessibility and i18n output"; echo; echo "Messages $(jq -r .messages "$OUT/out.json"); accessible $(jq -r .accessible "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "a11y-i18n-output worker: all_accessible=$(jq -r .all_accessible "$OUT/out.json") all_localizable=$(jq -r .all_localizable "$OUT/out.json")"
