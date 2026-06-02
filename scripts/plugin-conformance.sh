#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/plugin-conformance-gate.json}"; OUT="${2:-results/generated/plugin-conformance}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.plugin-conformance-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .required_methods as $R | .plugin as $P | .bad_plugin as $B
  | ([ $R[] | . as $m | ($P.methods|index($m))!=null ]|all) as $impl
  | ([ $R[] | . as $m | ($B.methods|index($m))!=null ]|all) as $bimpl
  | {version:"patchline.plugin-conformance/v1",
     required:($R|length), implements_all:$impl,
     conformance_rate:(($P.cases_passed/$P.cases_total)|r4),
     conforms:($impl and ($P.cases_passed==$P.cases_total)),
     bad_conforms:($bimpl and ($B.cases_passed==$B.cases_total))}

' "$SPEC" > "$OUT/out.json"
{ echo "# Plugin conformance suite"; echo; echo "Implements all $(jq -r .implements_all "$OUT/out.json"); rate $(jq -r .conformance_rate "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "plugin-conformance worker: conforms=$(jq -r .conforms "$OUT/out.json")"
