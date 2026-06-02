#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/fuzzing-harness-gate.json}"; OUT="${2:-results/generated/fuzzing-harness}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.fuzzing-harness-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .mutants as $M | .planted_unsound as $P
  | ([ $M[] | select(.crashed) ]|length) as $cr
  | ([ $M[] | select(.unsound_pass) ]|length) as $un
  | {version:"patchline.fuzzing-harness/v1",
     mutants:($M|length), crashes:$cr, unsound_passes:$un,
     survival_rate:((($M|length)-$cr)/($M|length)|r4),
     no_crash:($cr==0), sound:($un==0),
     planted_detected:($P.unsound_pass==true)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Migration fuzzing harness"; echo; echo "Mutants $(jq -r .mutants "$OUT/out.json"); crashes $(jq -r .crashes "$OUT/out.json"); unsound $(jq -r .unsound_passes "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "fuzzing-harness worker: no_crash=$(jq -r .no_crash "$OUT/out.json") sound=$(jq -r .sound "$OUT/out.json")"
