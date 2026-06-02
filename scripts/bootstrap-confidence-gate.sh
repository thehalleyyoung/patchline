#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/bootstrap-confidence-gates.json}"
OUT="${2:-results/generated/bootstrap-confidence-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.bootstrap-confidence-gates/v1" and
  .confidence_level == 0.95 and
  ([.metrics[].area] | index("ranking")) and
  ([.metrics[].area] | index("linking")) and
  ([.metrics[].area] | index("generated-check")) and
  ([.metrics[].area] | index("runtime")) and
  ([.metrics[].area] | index("review-burden"))
' "$SPEC" > /dev/null

bash scripts/research-question-gate.sh examples/research-questions.json "$OUT/research-questions" > "$OUT/research-question-gate.log"

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rq "$OUT/research-questions/summary.json" \
  '
  def safe_div($n; $d): if ($d // 0) == 0 then 0 else ($n / $d) end;
  def metric_value($id; $row):
    if $id == "ranking_stable_id_rate" then safe_div(($row.stable_risk_ids // 0); ($row.ranked_risks // 0))
    elif $id == "linking_links_per_ranked_risk" then safe_div(($row.evidence_links // 0); ($row.ranked_risks // 0))
    elif $id == "generated_check_pass_rate" then safe_div(($row.patchline_checks_passed // 0); (($row.patchline_checks_passed // 0) + ($row.patchline_checks_failed // 0)))
    elif $id == "runtime_seconds" then ($row.runtime_seconds // 0)
    elif $id == "review_burden_items" then ($row.review_burden_items // 0)
    else empty end;
  def mean: if length == 0 then 0 else add / length end;
  def percentile($q): .[((length - 1) * $q | floor)];
  def resamples($rows; $k):
    if $k == 0 then [[]]
    else [resamples($rows; $k - 1)[] as $tail | $rows[] as $row | [$row] + $tail]
    end;
  $spec[0] as $specdoc |
  $rq[0].slices as $rows |
  resamples($rows; ($rows | length)) as $samples |
  {
    version:"patchline.bootstrap-confidence-results/v1",
    source:"research-question-gate",
    confidence_level:$specdoc.confidence_level,
    method:$specdoc.method,
    slices:[$rows[] | {id, repo, subpath}],
    resample_count:($samples | length),
    metrics:[
      $specdoc.metrics[] as $metric |
      ([ $rows[] | metric_value($metric.id; .) ]) as $observations |
      ([ $samples[] | ([.[] | metric_value($metric.id; .)] | mean) ] | sort) as $estimates |
      {
        id:$metric.id,
        area:$metric.area,
        direction:$metric.direction,
        units:$metric.units,
        description:$metric.description,
        observations:$observations,
        point_estimate:($observations | mean),
        ci_lower:($estimates | percentile(0.025)),
        ci_upper:($estimates | percentile(0.975))
      }
    ]
  }' > "$OUT/bootstrap-confidence.json"

jq -e '
  .version == "patchline.bootstrap-confidence-results/v1" and
  .confidence_level == 0.95 and
  (.slices | length) >= 4 and
  .resample_count == 256 and
  ([.metrics[].area] | index("ranking")) and
  ([.metrics[].area] | index("linking")) and
  ([.metrics[].area] | index("generated-check")) and
  ([.metrics[].area] | index("runtime")) and
  ([.metrics[].area] | index("review-burden")) and
  all(.metrics[]; (.observations | length) >= 4 and .ci_lower <= .point_estimate and .point_estimate <= .ci_upper)
' "$OUT/bootstrap-confidence.json" > /dev/null

echo "bootstrap confidence gate passed: $(jq '.metrics | length' "$OUT/bootstrap-confidence.json") metrics, $(jq '.resample_count' "$OUT/bootstrap-confidence.json") deterministic resamples"
