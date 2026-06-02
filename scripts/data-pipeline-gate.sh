#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/data-pipeline-gate.json}"
OUT="${2:-results/generated/data-pipeline-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.data-pipeline-gate/v1" and (.frameworks|length) == 4' "$SPEC" > /dev/null

for phrase in "Airflow" "dbt" "Spark" "Kafka" "destructive" "make data-pipeline-gate"; do
  grep -F "$phrase" docs/data-pipeline.md README.md > /dev/null
done

bash scripts/data-pipeline.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in data-pipeline.json data-pipeline.md README.md data-pipelines.json; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.data-pipeline/v1" and
  .real_repo_detected == true and
  .framework_matrix_verified == true and
  .destructive_changes >= 5 and
  .airflow_findings >= 1 and
  .dbt_findings >= 1 and
  .spark_findings >= 1 and
  (.frameworks | length) == 4
' "$OUT/data-pipeline.json" > /dev/null

# Independently confirm at least one destructive spark overwrite is present on the real repo.
overwrites="$(jq '[.[] | select(.kind == "spark:writeOverwriteOrTable")] | length' "$OUT/data-pipelines.json")"
if [ "$overwrites" -lt 1 ]; then echo "no destructive spark overwrite detected"; exit 1; fi

jq -n --slurpfile r "$OUT/data-pipeline.json" '{
  version: "patchline.data-pipeline-gate-results/v1",
  real_repo: $r[0].real_repo,
  destructive_changes: $r[0].destructive_changes,
  frameworks: $r[0].frameworks,
  verified: true
}' > "$OUT/gate-summary.json"

echo "data-pipeline gate passed: multi-framework destructive pipeline detected on real repo, 4-framework matrix verified"
