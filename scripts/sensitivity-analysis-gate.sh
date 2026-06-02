#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/sensitivity-analysis.json}"
OUT="${2:-results/generated/sensitivity-analysis-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.sensitivity-analysis/v1" and
  .minimum_public_slices >= 4 and
  (.budget_settings | length) >= 3 and
  (.finding_caps | length) >= 3 and
  ([.link_thresholds[]] | index("identifier")) and
  ([.link_thresholds[]] | index("identifier+stage")) and
  ([.link_thresholds[]] | index("any")) and
  (.temporal_window_days | length) >= 3 and
  ([.risk_weight_settings[].id] | index("base")) and
  ([.risk_weight_settings[].id] | index("without-linked-evidence")) and
  ([.risk_weight_settings[].id] | index("safety-heavy"))
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"

  budget_rows=()
  while IFS=$'\t' read -r budget_id budget budget_risks; do
    budget_out="$case_out/budgets/$budget_id"
    mkdir -p "$budget_out"
    go run ./cmd/patchline repo propose --from-report "$case_out/analyze/baseline" --proposal-kind all --budget "$budget" --budget-risks "$budget_risks" --no-llm --out "$budget_out/proposal" --json > "$budget_out/proposal.json"
    go run ./cmd/patchline repo compare --before "$case_out/analyze/baseline" --after "$budget_out/proposal" --out "$budget_out/compare" --json > "$budget_out/compare.json"
    jq -n \
      --arg id "$budget_id" \
      --arg budget "$budget" \
      --argjson budget_risks "$budget_risks" \
      --slurpfile proposal "$budget_out/proposal.json" \
      --slurpfile compare "$budget_out/compare.json" \
      '{
        id:$id,
        budget:$budget,
        budget_risks:$budget_risks,
        generated_files:($proposal[0].generated_files | length),
        targeted_risks:$compare[0].summary.targeted_risks,
        checks_passed:$compare[0].summary.patchline_checks_passed,
        checks_failed:$compare[0].summary.patchline_checks_failed,
        intervention_loops:$compare[0].summary.intervention_loops
      }' > "$budget_out/row.json"
    budget_rows+=("$budget_out/row.json")
  done < <(jq -r '.budget_settings[] | [.id, .budget, .budget_risks] | @tsv' "$SPEC")

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile spec "$SPEC" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    --slurpfile analyze "$case_out/analyze.json" \
    --slurpfile budgets <(jq -s '.' "${budget_rows[@]}") \
    '
    def parse_day: if test("^[0-9]{4}-[0-9]{2}-[0-9]{2}$") then strptime("%Y-%m-%d") | mktime else null end;
    def span_days($w):
      ($w.start | parse_day) as $start |
      ($w.end | parse_day) as $end |
      if $start == null or $end == null then null else ($end - $start) / 86400 end;
    def adjusted_score($setting; $risk):
      if $setting == "base" then ($risk.score // 0)
      elif $setting == "without-linked-evidence" then
        (($risk.score // 0) - ([($risk.factors // [])[] | select(.name == "linked-project-evidence") | .weight] | add // 0))
      elif $setting == "safety-heavy" then
        (($risk.score // 0) + ([($risk.factors // [])[] | select((.name == "missing-transaction-boundary") or (.name == "missing-idempotency") or (.name == "retry-hazard") or (.name == "weak-rollback-signal")) | .weight] | add // 0))
      else ($risk.score // 0) end;
    $baseline[0] as $b |
    {
      id:$id,
      repo:$repo,
      subpath:$subpath,
      ranked_risks:$b.summary.ranked_risks,
      evidence_links:$b.summary.evidence_links,
      temporal_windows:$b.summary.temporal_windows,
      budget_sensitivity:$budgets[0],
      finding_cap_sensitivity:[
        $spec[0].finding_caps[] as $cap |
        {cap:$cap, reported:([ $b.risks[0:$cap][]? ] | length), hidden:(([$b.risks[]] | length) - ([ $b.risks[0:$cap][]? ] | length))}
      ],
      link_threshold_sensitivity:[
        $spec[0].link_thresholds[] as $threshold |
        {
          threshold:$threshold,
          retained_links:(
            if $threshold == "identifier" then [$b.evidence_links[]? | select(.confidence == "identifier")] | length
            elif $threshold == "identifier+stage" then [$b.evidence_links[]? | select((.identifiers | length) > 0 and (.fact_kind | length) > 0)] | length
            else [$b.evidence_links[]?] | length end
          )
        }
      ],
      temporal_window_sensitivity:[
        $spec[0].temporal_window_days[] as $days |
        {
          max_days:$days,
          retained_windows:([$b.temporal_windows[]? | span_days(.) as $span | select($span != null and $span <= $days)] | length),
          non_calendar_windows:([$b.temporal_windows[]? | span_days(.) as $span | select($span == null)] | length)
        }
      ],
      risk_weight_sensitivity:[
        $spec[0].risk_weight_settings[] as $setting |
        ([ $b.risks[0:400][]? | adjusted_score($setting.id; .) ]) as $scores |
        {
          setting:$setting.id,
          evaluated_risks:($scores | length),
          mean_adjusted_score:(if ($scores | length) == 0 then 0 else (($scores | add) / ($scores | length)) end),
          high_or_higher:([$scores[] | select(. >= 100)] | length)
        }
      ],
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.sensitivity-analysis-results/v1",
    settings:$spec[0],
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      budget_settings:($spec[0].budget_settings | length),
      finding_caps:($spec[0].finding_caps | length),
      link_thresholds:($spec[0].link_thresholds | length),
      temporal_window_settings:($spec[0].temporal_window_days | length),
      risk_weight_settings:($spec[0].risk_weight_settings | length)
    }
  }' > "$OUT/sensitivity-analysis.json"

jq -e '
  .version == "patchline.sensitivity-analysis-results/v1" and
  .summary.public_slices >= .settings.minimum_public_slices and
  . as $root |
  all($root.slices[];
    .verified == true and
    .ranked_risks > 0 and
    .evidence_links > 0 and
    .temporal_windows > 0 and
    (.budget_sensitivity | length) == ($root.settings.budget_settings | length) and
    (.finding_cap_sensitivity | length) == ($root.settings.finding_caps | length) and
    (.link_threshold_sensitivity | length) == ($root.settings.link_thresholds | length) and
    (.temporal_window_sensitivity | length) == ($root.settings.temporal_window_days | length) and
    (.risk_weight_sensitivity | length) == ($root.settings.risk_weight_settings | length) and
    all(.budget_sensitivity[]; .generated_files > 0 and .checks_failed == 0) and
    all(.finding_cap_sensitivity[]; .reported > 0 and .hidden >= 0) and
    .evidence_links as $evidence_links |
    any(.link_threshold_sensitivity[]; .threshold == "any" and .retained_links == $evidence_links) and
    all(.risk_weight_sensitivity[]; .evaluated_risks > 0)
  )
' "$OUT/sensitivity-analysis.json" > /dev/null

echo "sensitivity analysis gate passed: $(jq '.summary.public_slices' "$OUT/sensitivity-analysis.json") public slices across budget, cap, link, temporal, and risk-weight settings"
