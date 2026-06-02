#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/localized-quickstarts-gate.json}"; OUT="${2:-results/generated/localized-quickstarts}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.localized-quickstarts-gate/v1"' "$SPEC" > /dev/null
jq '

  .canonical_steps as $C | .locales as $L | .incomplete_locale as $I
  | ([ $L[] | . as $loc | ([ $C[] | . as $s | ($loc.steps|index($s))!=null ]|all) ]|all) as $parity
  | ([ $C[] | . as $s | ($I.steps|index($s))!=null ]|all) as $iparity
  | {version:"patchline.localized-quickstarts/v1",
     locales:($L|length), canonical_steps:($C|length),
     full_parity:$parity,
     incomplete_parity:$iparity}

' "$SPEC" > "$OUT/out.json"
{ echo "# Localized quickstarts"; echo; echo "Locales $(jq -r .locales "$OUT/out.json"); full parity $(jq -r .full_parity "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "localized-quickstarts worker: full_parity=$(jq -r .full_parity "$OUT/out.json")"
