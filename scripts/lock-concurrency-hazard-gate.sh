#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/lock-concurrency-hazards.json}"
OUT="${2:-results/generated/lock-concurrency-hazard-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.lock-concurrency-hazards/v1" and .minimum_public_repos >= 4 and (.required_surfaces | index("migration_sql")) != null and (.required_surfaces | index("generated_script")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind all --budget files=6,lines=160,tokens=16000,changes=3 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.lock_concurrency_hazards > 0 and
    (.lock_concurrency_hazards | length) == .summary.lock_concurrency_hazards and
    any(.lock_concurrency_hazards[]; (.severity == "critical" or .severity == "high" or .severity == "medium" or .severity == "low") and (.surface | length) > 0 and (.markers | length) > 0)
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -e '
    .summary.lock_concurrency_hazards > 0 and
    any(.lock_concurrency_hazards[]; .surface == "generated_script" and (.severity == "critical" or .severity == "high" or .severity == "medium" or .severity == "low"))
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
      baseline_hazards:$baseline[0].summary.lock_concurrency_hazards,
      baseline_critical:$baseline[0].summary.lock_hazard_critical,
      baseline_high:$baseline[0].summary.lock_hazard_high,
      baseline_medium:$baseline[0].summary.lock_hazard_medium,
      baseline_low:$baseline[0].summary.lock_hazard_low,
      generated_hazards:$compare[0].summary.lock_concurrency_hazards,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.lock-concurrency-hazards-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      baseline_hazards:($rows[0] | map(.baseline_hazards // 0) | add),
      generated_hazards:($rows[0] | map(.generated_hazards // 0) | add),
      high_or_critical:($rows[0] | map((.baseline_high // 0) + (.baseline_critical // 0)) | add)
    }
  }' > "$OUT/lock-concurrency-hazards.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.baseline_hazards >= (.slices | length) and
  .summary.generated_hazards >= ($spec[0].slices | length) and
  .summary.high_or_critical > 0
' "$OUT/lock-concurrency-hazards.json" > /dev/null

echo "lock/concurrency hazard gate passed: $(jq '.summary.public_repos' "$OUT/lock-concurrency-hazards.json") public repos, $(jq '.summary.baseline_hazards' "$OUT/lock-concurrency-hazards.json") baseline hazards, $(jq '.summary.generated_hazards' "$OUT/lock-concurrency-hazards.json") generated hazards"
