#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/migration-age-stratification-gate.json}"
OUT="${2:-results/generated/migration-age-stratification-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.migration-age-stratification-gate/v1" and
  (.slices | length) >= .minimum_repositories and
  (.required_strata | length) == 4
' "$SPEC" > /dev/null

for phrase in "Migration-age stratification" "backfill-heavy" "schema-only" "risk density" "make migration-age-stratification-gate"; do
  grep -F "$phrase" docs/migration-age-stratification.md README.md > /dev/null
done

bash scripts/migration-age-stratification.sh "$SPEC" "$OUT" > "$OUT.run.log"

while read -r output; do
  test -s "$OUT/$output"
done < <(jq -r '.required_outputs[]' "$SPEC")

min_repos="$(jq '.minimum_repositories' "$SPEC")"
min_migs="$(jq '.minimum_migrations' "$SPEC")"
min_strata="$(jq '.minimum_strata_populated' "$SPEC")"

jq -e --argjson min_repos "$min_repos" --argjson min_migs "$min_migs" --argjson min_strata "$min_strata" '
  .version == "patchline.migration-age-stratification/v1" and
  .summary.repositories >= $min_repos and
  .summary.migrations >= $min_migs and
  .summary.strata_populated >= $min_strata and
  .summary.backfill_heavy > 0 and
  .summary.schema_only > 0 and
  .summary.verified == true and
  (.strata | length) == 4 and
  (.cross_tab | length) == 4 and
  all(.strata[]; .migrations >= 0 and .total_risks >= 0) and
  ([.strata[] | select(.migrations > 0)] | length) >= $min_strata
' "$OUT/migration-age-stratification.json" > /dev/null

# Every required stratum name must be present in the report.
while read -r name; do
  jq -e --arg n "$name" 'any(.strata[]; .stratum == $n)' "$OUT/migration-age-stratification.json" > /dev/null
done < <(jq -r '.required_strata[]' "$SPEC")

# Each migration row must carry a real age key and a valid change type.
test "$(wc -l < "$OUT/migrations.jsonl" | tr -d ' ')" -ge "$min_migs"
jq -e -s 'all(.[]; (.age_key | test("^[0-9]+$")) and (.change_type == "schema-only" or .change_type == "backfill-heavy") and (.age_band == "recent" or .age_band == "old"))' "$OUT/migrations.jsonl" > /dev/null

for repo in lobsters/lobsters django/django apache/airflow; do
  grep -F "$repo" "$OUT/migration-age-stratification.md" > /dev/null
done

jq -n \
  --slurpfile report "$OUT/migration-age-stratification.json" \
  '{
    version: "patchline.migration-age-stratification-gate-results/v1",
    repositories: $report[0].summary.repositories,
    migrations: $report[0].summary.migrations,
    backfill_heavy: $report[0].summary.backfill_heavy,
    schema_only: $report[0].summary.schema_only,
    strata_populated: $report[0].summary.strata_populated,
    verified: true
  }' > "$OUT/gate-summary.json"

echo "migration-age stratification gate passed: repos $(jq '.repositories' "$OUT/gate-summary.json"), migrations $(jq '.migrations' "$OUT/gate-summary.json"), backfill-heavy $(jq '.backfill_heavy' "$OUT/gate-summary.json"), schema-only $(jq '.schema_only' "$OUT/gate-summary.json")"
