#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/data-volume-runtime-gate.json}"
OUT="${2:-results/generated/data-volume-runtime}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/reports"

jq -e '.version == "patchline.data-volume-runtime-gate/v1" and (.claim|length) > 200 and (.cases|length) >= .minimum_estimates' "$SPEC" > /dev/null

for phrase in "data-volume-aware runtime estimates" "make data-volume-runtime-gate"; do
  grep -F "$phrase" docs/db-version-semantics.md README.md > /dev/null
done

go test ./internal/dbsemantics ./cmd/patchline -run 'Test(RuntimeEstimates|RuntimeHints|DBSemanticsCommandWritesRuntimeEstimate)' -count=1 > "$OUT/go-test.log"

rows=()
while IFS= read -r encoded; do
  case_json="$(printf '%s' "$encoded" | base64 --decode)"
  id="$(jq -r '.id' <<<"$case_json")"
  engine="$(jq -r '.engine' <<<"$case_json")"
  version="$(jq -r '.version' <<<"$case_json")"
  case_sql="$OUT/cases/$id.sql"
  report="$OUT/reports/$id.json"
  repeat="$OUT/reports/$id.repeat.json"
  jq -r '.sql' <<<"$case_json" > "$case_sql"

  args=(go run ./cmd/patchline db-semantics --engine "$engine" --version "$version" --sql "$case_sql" --out "$report" --json)
  repeat_args=(go run ./cmd/patchline db-semantics --engine "$engine" --version "$version" --sql "$case_sql" --out "$repeat" --json)
  if jq -e '.table_hints != null' <<<"$case_json" > /dev/null; then
    hints="$OUT/cases/$id.hints.json"
    jq '{version:"patchline.data-volume-runtime-hints/v1", tables:.table_hints}' <<<"$case_json" > "$hints"
    args+=(--table-hints "$hints")
    repeat_args+=(--table-hints "$hints")
  fi

  "${args[@]}" > "$OUT/reports/$id.stdout.json"
  "${repeat_args[@]}" > "$OUT/reports/$id.repeat.stdout.json"
  jq -e --slurpfile repeat "$repeat" '.hash == $repeat[0].hash' "$report" > /dev/null

  expect_estimate="$(jq -r '.expect.estimate' <<<"$case_json")"
  if [[ "$expect_estimate" == "true" ]]; then
    jq -e \
      --arg class "$(jq -r '.expect.class' <<<"$case_json")" \
      --arg severity "$(jq -r '.expect.severity' <<<"$case_json")" \
      --arg duration "$(jq -r '.expect.duration' <<<"$case_json")" \
      --arg source_kind "$(jq -r '.expect.source_kind' <<<"$case_json")" \
      --arg table "$(jq -r '.expect.table' <<<"$case_json")" \
      --argjson min_rows "$(jq -r '.expect.min_rows' <<<"$case_json")" \
      '.summary.runtime_estimates == 1 and
       .statements[0].runtime_estimate.class == $class and
       .statements[0].runtime_estimate.severity == $severity and
       .statements[0].runtime_estimate.estimated_duration_class == $duration and
       .statements[0].runtime_estimate.source_kind == $source_kind and
       .statements[0].runtime_estimate.table == $table and
       .statements[0].runtime_estimate.rows_upper_bound >= $min_rows and
       (.statements[0].runtime_estimate.hint_hash|length) > 20 and
       (.statements[0].runtime_estimate.evidence|length) >= 3 and
       (.statements[0].runtime_estimate.obligations|length) >= 3 and
       any(.statements[0].rules[]?; .id == ("runtime." + $class))' \
      "$report" > /dev/null
  else
    jq -e '(.summary.runtime_estimates // 0) == 0 and (.statements[0].runtime_estimate? == null)' "$report" > /dev/null
  fi

  jq -n \
    --arg id "$id" \
    --arg engine "$engine" \
    --slurpfile report "$report" \
    '{
      id:$id,
      engine:$engine,
      estimate:(($report[0].summary.runtime_estimates // 0) == 1),
      class:($report[0].statements[0].runtime_estimate.class // "none"),
      severity:($report[0].statements[0].runtime_estimate.severity // "none"),
      duration:($report[0].statements[0].runtime_estimate.estimated_duration_class // "none"),
      source_kind:($report[0].statements[0].runtime_estimate.source_kind // "none"),
      rows:($report[0].statements[0].runtime_estimate.rows_upper_bound // 0),
      deterministic:true
    }' > "$OUT/reports/$id.row.json"
  rows+=("$OUT/reports/$id.row.json")
done < <(jq -r '.cases[] | @base64' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.data-volume-runtime-results/v1",
    claim:$spec[0].claim,
    cases:$rows[0],
    summary:{
      cases:($rows[0]|length),
      estimates:($rows[0] | map(select(.estimate == true)) | length),
      controls:($rows[0] | map(select(.estimate == false)) | length),
      high:($rows[0] | map(select(.severity == "high")) | length),
      deterministic:($rows[0] | map(select(.deterministic == true)) | length),
      classes:($rows[0] | map(select(.estimate == true) | .class) | unique),
      source_kinds:($rows[0] | map(select(.estimate == true) | .source_kind) | unique)
    }
  }' > "$OUT/data-volume-runtime.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.estimates >= $spec[0].minimum_estimates and
  .summary.controls >= 2 and
  .summary.high >= 2 and
  .summary.deterministic == .summary.cases and
  (.summary.classes | index("table_rewrite_estimate")) and
  (.summary.classes | index("online_schema_change_copy_estimate")) and
  (.summary.classes | index("table_replacement_estimate")) and
  (.summary.source_kinds | index("fixture")) and
  (.summary.source_kinds | index("public_statistic"))
' "$OUT/data-volume-runtime.json" > /dev/null

{
  echo "# Data-volume-aware runtime estimates"
  echo
  echo "| Case | Engine | Class | Severity | Duration | Rows | Source |"
  echo "|---|---|---|---|---|---:|---|"
  jq -r '.cases[] | "| \(.id) | \(.engine) | \(.class) | \(.severity) | \(.duration) | \(.rows) | \(.source_kind) |"' "$OUT/data-volume-runtime.json"
} > "$OUT/data-volume-runtime.md"
cp "$OUT/data-volume-runtime.md" "$OUT/README.md"

echo "data-volume runtime gate passed: $(jq -r '.summary.estimates' "$OUT/data-volume-runtime.json") estimates, $(jq -r '.summary.controls' "$OUT/data-volume-runtime.json") controls"
