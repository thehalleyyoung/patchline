#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/stratified-sampler-gate.json}"; OUT="${2:-results/generated/stratified-sampler}"
mkdir -p "$(dirname "$OUT")"
jq -e '.version == "patchline.stratified-sampler-gate/v1" and (.claim|length) > 200' "$SPEC" > /dev/null
for phrase in "stratum" "make stratified-sampler-gate"; do grep -F "$phrase" docs/stratified-sampler.md README.md > /dev/null; done
bash scripts/stratified-sampler.sh "$SPEC" "$OUT" > "$OUT.run.log"
jq -e '
  .version == "patchline.stratified-sampler/v1" and
  .covers_all == true and
  .rare_in_stratified == true and
  .rare_in_uniform == false and
  (.sample_strata | sort) == ["common","medium","rare"]
' "$OUT/strat.json" > /dev/null
jq -n --slurpfile r "$OUT/strat.json" '{version:"patchline.stratified-sampler-gate-results/v1", sample_ids:$r[0].sample_ids, covers_all:$r[0].covers_all, rare_missing_from_uniform:($r[0].rare_in_uniform|not), verified:true}' > "$OUT/gate-summary.json"
echo "stratified-sampler gate passed: stratified sample covers every stratum, uniform misses rare"
