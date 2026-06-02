#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/effect-size-reporting.json}"
OUT="${2:-results/generated/effect-size-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.effect-size-reporting/v1" and
  .source == "paired-statistical-tests" and
  ([.required_effect_sizes[]] | index("mean_delta")) and
  ([.required_effect_sizes[]] | index("median_delta")) and
  ([.required_effect_sizes[]] | index("relative_lift")) and
  ([.required_effect_sizes[]] | index("win_rate")) and
  ([.required_effect_sizes[]] | index("tie_rate")) and
  ([.required_effect_sizes[]] | index("standardized_paired_delta")) and
  (.interpretation_policy | contains("magnitude"))
' "$SPEC" > /dev/null

bash scripts/paired-statistical-tests-gate.sh examples/paired-statistical-tests.json "$OUT/paired" > "$OUT/paired-statistical-tests-gate.log"

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile paired "$OUT/paired/paired-statistical-tests.json" \
  '
  def mean: if length == 0 then 0 else add / length end;
  def median:
    sort as $sorted |
    ($sorted | length) as $n |
    if $n == 0 then 0
    elif ($n % 2) == 1 then $sorted[($n / 2 | floor)]
    else (($sorted[($n / 2) - 1] + $sorted[($n / 2)]) / 2)
    end;
  def variance:
    . as $values |
    ($values | mean) as $m |
    if ($values | length) <= 1 then 0 else ($values | map((. - $m) * (. - $m)) | add) / (($values | length) - 1) end;
  def sqrt_newton:
    . as $x |
    if $x == 0 then 0
    else reduce range(0; 20) as $_ ($x; (. + ($x / .)) / 2)
    end;
  def relative_lift($patchline; $baseline):
    if $baseline == 0 then null else (($patchline - $baseline) / $baseline) end;
  def interpretation($mean_delta; $win_rate; $tie_rate):
    if $tie_rate == 1 then "no observed paired difference"
    elif $mean_delta > 0 and $win_rate >= 0.75 then "positive effect on most paired slices"
    elif $mean_delta > 0 then "positive average effect with mixed paired slices"
    elif $mean_delta == 0 then "zero average effect"
    else "negative average effect"
    end;
  $spec[0] as $specdoc |
  {
    version:"patchline.effect-size-results/v1",
    source:$specdoc.source,
    interpretation_policy:$specdoc.interpretation_policy,
    public_slices:$paired[0].public_slices,
    effects:[
      $paired[0].tests[] as $test |
      ($test.pairs | map(.diff)) as $diffs |
      ($diffs | variance | sqrt_newton) as $sd |
      (($test.sign_test.wins + $test.sign_test.losses + $test.sign_test.ties) | if . == 0 then 1 else . end) as $pairs |
      {
        id:$test.id,
        metric:$test.metric,
        mean_patchline:$test.mean_patchline,
        mean_baseline:$test.mean_baseline,
        exact_sign_test_p_value:$test.sign_test.p_value,
        effect_sizes:{
          mean_delta:($diffs | mean),
          median_delta:($diffs | median),
          relative_lift:relative_lift($test.mean_patchline; $test.mean_baseline),
          win_rate:($test.sign_test.wins / $pairs),
          tie_rate:($test.sign_test.ties / $pairs),
          standardized_paired_delta:(if $sd == 0 then null else (($diffs | mean) / $sd) end)
        },
        interpretation:interpretation(($diffs | mean); ($test.sign_test.wins / $pairs); ($test.sign_test.ties / $pairs)),
        pairs:$test.pairs
      }
    ]
  }' > "$OUT/effect-sizes.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.effect-size-results/v1" and
  .source == "paired-statistical-tests" and
  .public_slices >= 4 and
  (.effects | length) >= 5 and
  all(.effects[];
    (.effect_sizes | has("mean_delta")) and
    (.effect_sizes | has("median_delta")) and
    (.effect_sizes | has("relative_lift")) and
    (.effect_sizes | has("win_rate")) and
    (.effect_sizes | has("tie_rate")) and
    (.effect_sizes | has("standardized_paired_delta")) and
    (.interpretation | length) > 0 and
    (.pairs | length) >= 4
  ) and
  any(.effects[]; .effect_sizes.win_rate == 1 and .effect_sizes.mean_delta > 0) and
  any(.effects[]; .effect_sizes.tie_rate == 1 and .interpretation == "no observed paired difference")
' "$OUT/effect-sizes.json" > /dev/null

echo "effect-size gate passed: $(jq '.effects | length' "$OUT/effect-sizes.json") comparisons report magnitude beyond p-values"
