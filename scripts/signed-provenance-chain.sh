#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/signed-provenance-chain-gate.json}"; OUT="${2:-results/generated/signed-provenance-chain}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.signed-provenance-chain-gate/v1"' "$SPEC" > /dev/null
jq '

  .chain as $c | .broken_chain as $b
  | ([ range(1;($c|length)) as $i | ($c[$i].prev == $c[$i-1].digest) ] | all) as $intact
  | ([ $c[] | .signed ] | all) as $allsigned
  | ([ range(1;($b|length)) as $i | ($b[$i].prev == $b[$i-1].digest) ] | all) as $bok
  | {version:"patchline.signed-provenance-chain/v1",
     length:($c|length), intact:$intact, all_signed:$allsigned,
     terminal:($c[-1].stage), broken_intact:$bok}

' "$SPEC" > "$OUT/out.json"
{ echo "# Signed provenance chain"; echo; echo "Chain length $(jq -r .length "$OUT/out.json"); intact $(jq -r .intact "$OUT/out.json"); signed $(jq -r .all_signed "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "signed-provenance-chain worker: intact=$(jq -r .intact "$OUT/out.json") signed=$(jq -r .all_signed "$OUT/out.json")"
