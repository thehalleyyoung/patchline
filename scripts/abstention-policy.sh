#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/abstention-policy-gate.json}"; OUT="${2:-results/generated/abstention-policy}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.abstention-policy-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .cases as $C | .abstain_below as $t | .accuracy_floor as $floor
  | ([ $C[] | select(.conf >= $t) ]) as $decided
  | ([ $decided[] | select(.correct) ]|length) as $dc
  | ([ $C[] | select(.correct) ]|length) as $fullc
  | {version:"patchline.abstention-policy/v1",
     total:($C|length), decided:($decided|length),
     coverage:((($decided|length)/($C|length))|r4),
     selective_accuracy:(($dc/($decided|length))|r4),
     meets_floor:((($dc/($decided|length))) >= $floor),
     full_coverage_meets_floor:((($fullc/($C|length))) >= $floor)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Uncertainty-aware abstention policy"; echo; echo "Coverage $(jq -r .coverage "$OUT/out.json"); selective acc $(jq -r .selective_accuracy "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "abstention-policy worker: selective_accuracy=$(jq -r .selective_accuracy "$OUT/out.json") meets_floor=$(jq -r .meets_floor "$OUT/out.json")"
