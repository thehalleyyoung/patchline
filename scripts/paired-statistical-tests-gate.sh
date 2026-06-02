#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/paired-statistical-tests.json}"
OUT="${2:-results/generated/paired-statistical-tests-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.paired-statistical-tests/v1" and
  .method == "exact two-sided paired sign test" and
  .minimum_public_slices >= 4 and
  ([.comparisons[].id] | index("grep-only")) and
  ([.comparisons[].id] | index("sql-only")) and
  ([.comparisons[].id] | index("identifier-only")) and
  ([.comparisons[].id] | index("temporal-only")) and
  ([.comparisons[].id] | index("no-facts-generation"))
' "$SPEC" > /dev/null

bash scripts/four-repo-analysis-demo.sh "$OUT/matrix" > "$OUT/four-repo-analysis-demo.log"

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile matrix "$OUT/matrix/summary.json" \
  '
  def choose($n; $k):
    if $k < 0 or $k > $n then 0
    elif $k == 0 or $k == $n then 1
    else reduce range(1; ($k + 1)) as $i (1; . * (($n - $k + $i) / $i))
    end;
  def pow2($n): reduce range(0; $n) as $_ (1; . * 2);
  def sum_binomial_through($n; $k): [range(0; ($k + 1)) | choose($n; .)] | add;
  def sign_test($diffs):
    ($diffs | map(select(. != 0))) as $nonzero |
    ($nonzero | map(select(. > 0)) | length) as $wins |
    ($nonzero | map(select(. < 0)) | length) as $losses |
    ($nonzero | length) as $n |
    (if $wins < $losses then $wins else $losses end) as $tail |
    (if $n == 0 then 1 else ([1, ((2 * sum_binomial_through($n; $tail)) / pow2($n))] | min) end) as $p |
    {n:$n, wins:$wins, losses:$losses, ties:($diffs | length - $n), p_value:$p};
  def patchline_value($id; $row):
    if $id == "grep-only" then $row.comparisons.grep_only_risk_detection.patchline_ranked_risks
    elif $id == "sql-only" then $row.comparisons.sql_only_without_links.patchline_ranked_risks
    elif $id == "identifier-only" then $row.comparisons.identifier_only_without_temporal.patchline_evidence_links
    elif $id == "temporal-only" then $row.comparisons.temporal_only_without_identifiers.patchline_evidence_links
    elif $id == "no-facts-generation" then
      if (($row.fact_grounded_generation_comparison.fact_grounded_prompt_hash != $row.fact_grounded_generation_comparison.without_facts_prompt_hash) and
          ($row.fact_grounded_generation_comparison.fact_grounded_output_hash != $row.fact_grounded_generation_comparison.without_facts_output_hash)) then 1 else 0 end
    else empty end;
  def baseline_value($id; $row):
    if $id == "grep-only" then $row.comparisons.grep_only_risk_detection.grep_only_matches
    elif $id == "sql-only" then $row.comparisons.sql_only_without_links.sql_only_ranked_risks
    elif $id == "identifier-only" then $row.comparisons.identifier_only_without_temporal.identifier_only_links
    elif $id == "temporal-only" then $row.comparisons.temporal_only_without_identifiers.temporal_or_date_only_links
    elif $id == "no-facts-generation" then 0
    else empty end;
  def mean: if length == 0 then 0 else add / length end;
  $spec[0] as $specdoc |
  $matrix[0].cases as $rows |
  {
    version:"patchline.paired-statistical-test-results/v1",
    method:$specdoc.method,
    public_slices:($rows | length),
    tests:[
      $specdoc.comparisons[] as $comparison |
      ([ $rows[] | {
        id:(.label + " " + .repo + ":" + .subpath),
        patchline:(patchline_value($comparison.id; .)),
        baseline:(baseline_value($comparison.id; .)),
        diff:((patchline_value($comparison.id; .)) - (baseline_value($comparison.id; .)))
      }]) as $pairs |
      ($pairs | map(.diff)) as $diffs |
      (sign_test($diffs)) as $test |
      {
        id:$comparison.id,
        metric:$comparison.metric,
        alternative:$comparison.alternative,
        pairs:$pairs,
        mean_patchline:($pairs | map(.patchline) | mean),
        mean_baseline:($pairs | map(.baseline) | mean),
        mean_delta:($diffs | mean),
        sign_test:$test
      }
    ]
  }' > "$OUT/paired-statistical-tests.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.paired-statistical-test-results/v1" and
  .method == "exact two-sided paired sign test" and
  .public_slices >= $spec[0].minimum_public_slices and
  (.tests | length) == ($spec[0].comparisons | length) and
  all(.tests[]; (.pairs | length) >= 4 and ((.sign_test.n + .sign_test.ties) == (.pairs | length)) and .sign_test.p_value >= 0 and .sign_test.p_value <= 1) and
  any(.tests[]; .sign_test.n >= 4 and .sign_test.wins == .sign_test.n)
' "$OUT/paired-statistical-tests.json" > /dev/null

echo "paired statistical tests gate passed: $(jq '.tests | length' "$OUT/paired-statistical-tests.json") exact sign tests over $(jq '.public_slices' "$OUT/paired-statistical-tests.json") public slices"
