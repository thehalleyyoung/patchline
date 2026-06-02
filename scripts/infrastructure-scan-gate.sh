#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/infrastructure-scan-gate.json}"
OUT="${2:-results/generated/infrastructure-scan-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.infrastructure-scan-gate/v1" and .minimum_public_repos >= 4 and (.required_categories | length) == 5' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref path; do
  case_out="$OUT/cases/$id"
  case_root="$case_out/repo"
  mkdir -p "$case_root/$(dirname "$path")"
  gh api "repos/$repo/contents/$path?ref=$ref" --jq '.content' | base64 --decode > "$case_root/$path"
  go run ./cmd/patchline repo inventory "$case_root" --out "$case_out/inventory" --json > "$case_out/inventory.json"
  go run ./cmd/patchline intake "$case_root" --out "$case_out/intake" --json > "$case_out/intake.json"
  go run ./cmd/patchline repo baseline --inventory "$case_out/inventory" --intake "$case_out/intake" --json > "$case_out/baseline.json"
  jq -e '.summary.infrastructure_findings > 0 and (.infrastructure_findings | length) > 0' "$case_out/baseline.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg path "$path" \
    --slurpfile inventory "$case_out/inventory.json" \
    --slurpfile baseline "$case_out/baseline.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      path:$path,
      files_scanned:$inventory[0].files_scanned,
      findings:$baseline[0].summary.infrastructure_findings,
      database_jobs:$baseline[0].summary.infrastructure_database_jobs,
      migration_jobs:$baseline[0].summary.infrastructure_migration_jobs,
      cron_repairs:$baseline[0].summary.infrastructure_cron_repairs,
      secret_references:$baseline[0].summary.infrastructure_secret_references,
      deploy_ordering:$baseline[0].summary.infrastructure_deploy_ordering,
      kinds:($baseline[0].infrastructure_findings | map(.kind) | unique),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .path] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.infrastructure-scan-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      findings:($rows[0] | map(.findings) | add),
      database_jobs:($rows[0] | map(.database_jobs) | add),
      migration_jobs:($rows[0] | map(.migration_jobs) | add),
      cron_repairs:($rows[0] | map(.cron_repairs) | add),
      secret_references:($rows[0] | map(.secret_references) | add),
      deploy_ordering:($rows[0] | map(.deploy_ordering) | add),
      kinds:($rows[0] | map(.kinds[]) | unique)
    }
  }' > "$OUT/infrastructure-scan-gate.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.database_jobs > 0 and
  .summary.migration_jobs > 0 and
  .summary.cron_repairs > 0 and
  .summary.secret_references > 0 and
  .summary.deploy_ordering > 0
' "$OUT/infrastructure-scan-gate.json" > /dev/null

echo "Infrastructure scan gate passed: $(jq '.summary.public_repos' "$OUT/infrastructure-scan-gate.json") public repos, findings=$(jq '.summary.findings' "$OUT/infrastructure-scan-gate.json"), database_jobs=$(jq '.summary.database_jobs' "$OUT/infrastructure-scan-gate.json"), migration_jobs=$(jq '.summary.migration_jobs' "$OUT/infrastructure-scan-gate.json"), cron_repairs=$(jq '.summary.cron_repairs' "$OUT/infrastructure-scan-gate.json"), secrets=$(jq '.summary.secret_references' "$OUT/infrastructure-scan-gate.json"), deploy_ordering=$(jq '.summary.deploy_ordering' "$OUT/infrastructure-scan-gate.json")"
