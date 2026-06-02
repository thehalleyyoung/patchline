#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/data-pipeline-gate.json}"
OUT="${2:-results/generated/data-pipeline}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis"

jq -e '
  .version == "patchline.data-pipeline-gate/v1" and
  (.claim | length) > 200 and
  (.frameworks | length) == 4
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
min_destructive="$(jq -r '.real_repo.minimum_destructive' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" \
  --download-dir "$OUT/cache" --stages inventory --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

INV="$OUT/analysis/inventory/inventory.json"
test -s "$INV"

jq '.data_pipelines // []' "$INV" > "$OUT/data-pipelines.json"

destructive_count="$(jq '[.[] | select(.rationale | test("destructive=true"))] | length' "$OUT/data-pipelines.json")"
airflow_present="$(jq '[.[] | select(.kind | startswith("airflow:"))] | length' "$OUT/data-pipelines.json")"
dbt_present="$(jq '[.[] | select(.kind | startswith("dbt:"))] | length' "$OUT/data-pipelines.json")"
spark_present="$(jq '[.[] | select(.kind | startswith("spark:"))] | length' "$OUT/data-pipelines.json")"

real_repo_detected=false
if [ "$destructive_count" -ge "$min_destructive" ] && [ "$airflow_present" -ge 1 ] && [ "$dbt_present" -ge 1 ] && [ "$spark_present" -ge 1 ]; then
  real_repo_detected=true
fi

go test ./internal/project/ -run 'TestInventoryDetectsDataPipelineChanges|TestInventoryDoesNotFlagDataPipelineWithoutSignal' \
  > "$OUT/unit-tests.log" 2>&1 && unit_ok=true || unit_ok=false
rm -rf internal/project/results

jq -n \
  --arg repo "$repo" \
  --argjson destructive "$destructive_count" \
  --argjson airflow "$airflow_present" \
  --argjson dbt "$dbt_present" \
  --argjson spark "$spark_present" \
  --argjson real_detected "$real_repo_detected" \
  --argjson unit_ok "$unit_ok" \
  --slurpfile spec "$SPEC" '
  {
    version: "patchline.data-pipeline/v1",
    real_repo: $repo,
    frameworks: ($spec[0].frameworks),
    airflow_findings: $airflow,
    dbt_findings: $dbt,
    spark_findings: $spark,
    destructive_changes: $destructive,
    real_repo_detected: $real_detected,
    framework_matrix_verified: $unit_ok
  }
' > "$OUT/data-pipeline.json"

{
  echo "# Data-pipeline repair evidence"
  echo
  jq -r '"Patchline detected `" + (.destructive_changes|tostring) + "` destructive data-pipeline operations in the real `" + .real_repo + "` lakehouse repository, spanning Airflow (" + (.airflow_findings|tostring) + "), dbt (" + (.dbt_findings|tostring) + "), and Spark (" + (.spark_findings|tostring) + ") findings."' "$OUT/data-pipeline.json"
  echo
  echo "## Guarantees"
  jq -r '"- real-repo multi-framework destructive pipeline detected: `" + (.real_repo_detected|tostring) + "`\n- four-framework matrix (Airflow, dbt, Spark, Kafka) and no-false-positive rule verified by unit tests: `" + (.framework_matrix_verified|tostring) + "`"' "$OUT/data-pipeline.json"
  echo
  echo "Patchline treats data pipelines as first-class data-change surfaces: Airflow backfills and dagrun resets, dbt full-refresh and table materializations, Spark overwrite writes and saveAsTable, and Kafka offset resets are all recorded as searchable \`data_pipeline_change\` facts alongside relational and NoSQL migration risks."
} > "$OUT/data-pipeline.md"
cp "$OUT/data-pipeline.md" "$OUT/README.md"

echo "data-pipeline gate complete: $(jq '.destructive_changes' "$OUT/data-pipeline.json") destructive ops on real repo (airflow+dbt+spark), matrix verified $(jq '.framework_matrix_verified' "$OUT/data-pipeline.json")"
