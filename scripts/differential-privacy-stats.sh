#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/differential-privacy-stats-gate.json}"; OUT="${2:-results/generated/differential-privacy-stats}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.differential-privacy-stats-gate/v1"' "$SPEC" > /dev/null
jq '

  def r4: (.*10000|round)/10000;
  .sensitivity as $s | .epsilon as $e | .true_count as $t | .released_count as $rel
  | ($s / $e | r4) as $scale
  | {version:"patchline.differential-privacy-stats/v1",
     epsilon:$e, sensitivity:$s, noise_scale:$scale,
     released:$rel, abs_error:(($rel-$t)|if .<0 then -. else . end),
     within_bound:((($rel-$t)|if .<0 then -. else . end) <= (10*$scale)),
     epsilon_valid:($e>0),
     bad_epsilon_valid:(.bad_epsilon>0)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Differentially private corpus statistics"; echo; echo "Epsilon $(jq -r .epsilon "$OUT/out.json"); noise scale $(jq -r .noise_scale "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "differential-privacy-stats worker: noise_scale=$(jq -r .noise_scale "$OUT/out.json") within_bound=$(jq -r .within_bound "$OUT/out.json")"
