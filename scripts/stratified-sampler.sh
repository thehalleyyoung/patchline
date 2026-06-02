#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/stratified-sampler-gate.json}"; OUT="${2:-results/generated/stratified-sampler}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.stratified-sampler-gate/v1" and (.population|length) >= 1' "$SPEC" > /dev/null
jq '
  .population as $P | .uniform_sample as $U
  | ([ $P[].stratum ] | unique) as $strata
  | ([ $strata[] as $s | ($P | map(select(.stratum == $s)) | first) ]) as $sample
  | ([ $sample[].stratum ] | unique) as $sample_strata
  | ([ $P[] | select([ .id == $U[] ] | any) | .stratum ] | unique) as $uniform_strata
  | {
      version: "patchline.stratified-sampler/v1",
      strata: $strata,
      sample_ids: ([ $sample[].id ]),
      sample_strata: $sample_strata,
      covers_all: (($strata - $sample_strata) == []),
      uniform_strata: $uniform_strata,
      rare_in_stratified: ([ "rare" == $sample_strata[] ] | any),
      rare_in_uniform: ([ "rare" == $uniform_strata[] ] | any)
    }
' "$SPEC" > "$OUT/strat.json"
{ echo "# Stratified sampler"; echo; echo "Strata: $(jq -rc '.strata' "$OUT/strat.json"); sample: $(jq -rc '.sample_ids' "$OUT/strat.json")"; echo "Covers all: $(jq -r '.covers_all' "$OUT/strat.json"); rare in uniform: $(jq -r '.rare_in_uniform' "$OUT/strat.json")"; } > "$OUT/strat.md"
cp "$OUT/strat.md" "$OUT/README.md"
echo "stratified-sampler worker: covers_all=$(jq -r '.covers_all' "$OUT/strat.json") rare_in_uniform=$(jq -r '.rare_in_uniform' "$OUT/strat.json")"
