#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/data-retention-privacy-hazards.json}"
OUT="${2:-results/generated/data-retention-privacy-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.data-retention-privacy-hazards/v1" and .minimum_public_repos >= 4 and (.required_surfaces | index("migration_sql")) != null and (.required_surfaces | index("generated_script")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind all --budget files=6,lines=160,tokens=16000,changes=3 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.data_retention_privacy_hazards > 0 and
    (.data_retention_privacy_hazards | length) == .summary.data_retention_privacy_hazards and
    any(.data_retention_privacy_hazards[]; (.severity == "critical" or .severity == "high" or .severity == "medium" or .severity == "low") and (.surface | length) > 0 and (.markers | length) > 0)
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -e '
    .summary.data_retention_privacy_hazards > 0 and
    any(.data_retention_privacy_hazards[]; .surface == "generated_script" and (.severity == "critical" or .severity == "high" or .severity == "medium" or .severity == "low"))
  ' "$case_out/analyze/compare/compare.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    --slurpfile compare "$case_out/analyze/compare/compare.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      kind:"repo-slice",
      baseline_hazards:$baseline[0].summary.data_retention_privacy_hazards,
      baseline_critical:$baseline[0].summary.privacy_hazard_critical,
      baseline_high:$baseline[0].summary.privacy_hazard_high,
      baseline_medium:$baseline[0].summary.privacy_hazard_medium,
      baseline_low:$baseline[0].summary.privacy_hazard_low,
      generated_hazards:$compare[0].summary.data_retention_privacy_hazards,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.data-retention-privacy-hazards-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      baseline_hazards:($rows[0] | map(.baseline_hazards // 0) | add),
      generated_hazards:($rows[0] | map(.generated_hazards // 0) | add),
      high_or_critical:($rows[0] | map((.baseline_high // 0) + (.baseline_critical // 0)) | add)
    }
  }' > "$OUT/data-retention-privacy-hazards.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.baseline_hazards >= (.slices | length) and
  .summary.generated_hazards >= ($spec[0].slices | length) and
  .summary.high_or_critical > 0
' "$OUT/data-retention-privacy-hazards.json" > /dev/null

echo "data-retention/privacy gate passed: $(jq '.summary.public_repos' "$OUT/data-retention-privacy-hazards.json") public repos, $(jq '.summary.baseline_hazards' "$OUT/data-retention-privacy-hazards.json") baseline hazards, $(jq '.summary.generated_hazards' "$OUT/data-retention-privacy-hazards.json") generated hazards"
