#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/query-plan-regression-gate.json}"
OUT="${2:-results/generated/query-plan-regression}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/reports"

jq -e '.version == "patchline.query-plan-regression-gate/v1" and (.claim|length) > 200 and (.cases|length) >= .minimum_findings' "$SPEC" > /dev/null

for phrase in "query-plan regression" "make query-plan-regression-gate"; do
  grep -F "$phrase" docs/query-plan-regression.md README.md > /dev/null
done

go test ./internal/dbsemantics ./cmd/patchline -run 'Test(QueryPlanRegression|DBSemanticsCommandWritesQueryPlanRegression)' -count=1 > "$OUT/go-test.log"

rows=()
while IFS= read -r encoded; do
  case_json="$(printf '%s' "$encoded" | base64 --decode)"
  id="$(jq -r '.id' <<<"$case_json")"
  engine="$(jq -r '.engine' <<<"$case_json")"
  version="$(jq -r '.version' <<<"$case_json")"
  case_sql="$OUT/cases/$id.sql"
  report="$OUT/reports/$id.json"
  jq -r '.sql' <<<"$case_json" > "$case_sql"

  go run ./cmd/patchline db-semantics --engine "$engine" --version "$version" --sql "$case_sql" --out "$report" --json > "$OUT/reports/$id.stdout.json"

  expect_finding="$(jq -r '.expect.finding' <<<"$case_json")"
  if [[ "$expect_finding" == "true" ]]; then
    jq -e \
      --arg class "$(jq -r '.expect.class' <<<"$case_json")" \
      --arg change_kind "$(jq -r '.expect.change_kind' <<<"$case_json")" \
      --arg table "$(jq -r '.expect.table' <<<"$case_json")" \
      --arg index "$(jq -r '.expect.index' <<<"$case_json")" \
      --argjson regressions "$(jq -r '.expect.regressions' <<<"$case_json")" \
      '.summary.query_plan_regression_checks == 1 and
       (.summary.query_plan_regressions // 0) == $regressions and
       .statements[0].query_plan_regression.class == $class and
       .statements[0].query_plan_regression.change_kind == $change_kind and
       .statements[0].query_plan_regression.table == $table and
       (.statements[0].query_plan_regression.index // "") == $index and
       (.statements[0].query_plan_regression.representative_workloads|length) >= 1 and
       (.statements[0].query_plan_regression.before_plans|length) == (.statements[0].query_plan_regression.representative_workloads|length) and
       (.statements[0].query_plan_regression.after_plans|length) == (.statements[0].query_plan_regression.representative_workloads|length) and
       (.statements[0].query_plan_regression.evidence|length) >= 2 and
       (.statements[0].query_plan_regression.obligations|length) >= 3 and
       (.statements[0].query_plan_regression.handoffs|length) >= 1 and
       any(.statements[0].rules[]?; .id == ("query_plan." + $class))' \
      "$report" > /dev/null
  else
    jq -e '(.summary.query_plan_regression_checks // 0) == 0 and (.summary.query_plan_regressions // 0) == 0 and (.statements[0].query_plan_regression? == null)' "$report" > /dev/null
  fi

  jq -n \
    --arg id "$id" \
    --arg engine "$engine" \
    --slurpfile report "$report" \
    '{
      id:$id,
      engine:$engine,
      finding:(($report[0].summary.query_plan_regression_checks // 0) == 1),
      class:($report[0].statements[0].query_plan_regression.class // "none"),
      change_kind:($report[0].statements[0].query_plan_regression.change_kind // "none"),
      regressions:($report[0].summary.query_plan_regressions // 0),
      workloads:($report[0].statements[0].query_plan_regression.representative_workloads // [] | length),
      verified:true
    }' > "$OUT/reports/$id.row.json"
  rows+=("$OUT/reports/$id.row.json")
done < <(jq -r '.cases[] | @base64' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.query-plan-regression-results/v1",
    claim:$spec[0].claim,
    cases:$rows[0],
    summary:{
      cases:($rows[0]|length),
      findings:($rows[0] | map(select(.finding == true)) | length),
      controls:($rows[0] | map(select(.finding == false)) | length),
      regressions:($rows[0] | map(.regressions) | add),
      verified:($rows[0] | map(select(.verified == true)) | length),
      classes:($rows[0] | map(select(.finding == true) | .class) | unique)
    }
  }' > "$OUT/query-plan-regression.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.findings >= $spec[0].minimum_findings and
  .summary.controls >= 1 and
  .summary.regressions >= 4 and
  .summary.verified == .summary.cases and
  (.summary.classes | index("index_addition_plan_check")) and
  (.summary.classes | index("index_drop_regression")) and
  (.summary.classes | index("column_shape_regression")) and
  (.summary.classes | index("column_drop_regression"))
' "$OUT/query-plan-regression.json" > /dev/null

{
  echo "# Query-plan regression preflight"
  echo
  echo "| Case | Engine | Class | Workloads | Regressions |"
  echo "|---|---|---|---:|---:|"
  jq -r '.cases[] | "| \(.id) | \(.engine) | \(.class) | \(.workloads) | \(.regressions) |"' "$OUT/query-plan-regression.json"
} > "$OUT/query-plan-regression.md"
cp "$OUT/query-plan-regression.md" "$OUT/README.md"

echo "query-plan regression gate passed: $(jq -r '.summary.findings' "$OUT/query-plan-regression.json") findings, $(jq -r '.summary.controls' "$OUT/query-plan-regression.json") control"
