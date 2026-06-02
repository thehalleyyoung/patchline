#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/idempotency-classification.json}"
OUT="${2:-results/generated/idempotency-classification-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.idempotency-classification/v1" and .minimum_public_repos >= 4 and (.required_surfaces | index("migration_sql")) != null and (.required_surfaces | index("generated_script")) != null and (.required_surfaces | index("runbook_command")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind all --budget files=6,lines=160,tokens=16000,changes=3 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.idempotency_classifications > 0 and
    (.idempotency_classifications | length) == .summary.idempotency_classifications and
    any(.idempotency_classifications[]; (.status == "proven" or .status == "guarded" or .status == "unknown" or .status == "non_idempotent") and (.surface | length) > 0)
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -e '
    .summary.idempotency_classifications > 0 and
    any(.idempotency_classifications[]; .surface == "generated_script" and (.status == "proven" or .status == "guarded" or .status == "unknown" or .status == "non_idempotent"))
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
      baseline_classes:$baseline[0].summary.idempotency_classifications,
      baseline_proven:$baseline[0].summary.idempotency_proven,
      baseline_guarded:$baseline[0].summary.idempotency_guarded,
      baseline_unknown:$baseline[0].summary.idempotency_unknown,
      baseline_non_idempotent:$baseline[0].summary.idempotency_non_idempotent,
      generated_classes:$compare[0].summary.idempotency_classifications,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

while IFS=$'\t' read -r id repo ref url; do
  case_out="$OUT/cases/$id"
  input="$case_out/input"
  mkdir -p "$input"
  curl -fsSL "$url" -o "$input/runbook.md"
  go run ./cmd/patchline repo analyze "$input" --stages inventory,baseline --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.idempotency_classifications > 0 and
    any(.idempotency_classifications[]; .surface == "runbook_command" and (.status == "proven" or .status == "guarded" or .status == "unknown" or .status == "non_idempotent"))
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      kind:"runbook",
      baseline_classes:$baseline[0].summary.idempotency_classifications,
      runbook_classes:($baseline[0].idempotency_classifications | map(select(.surface == "runbook_command")) | length),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.runbooks[] | [.id, .repo, .ref, .url] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.idempotency-classification-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      baseline_classes:($rows[0] | map(.baseline_classes // 0) | add),
      generated_classes:($rows[0] | map(.generated_classes // 0) | add),
      runbook_classes:($rows[0] | map(.runbook_classes // 0) | add)
    }
  }' > "$OUT/idempotency-classification.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.baseline_classes >= (.slices | length) and
  .summary.generated_classes >= ($spec[0].slices | length) and
  .summary.runbook_classes >= ($spec[0].runbooks | length)
' "$OUT/idempotency-classification.json" > /dev/null

echo "idempotency classification gate passed: $(jq '.summary.public_repos' "$OUT/idempotency-classification.json") public repos, $(jq '.summary.baseline_classes' "$OUT/idempotency-classification.json") baseline classes, $(jq '.summary.generated_classes' "$OUT/idempotency-classification.json") generated classes, $(jq '.summary.runbook_classes' "$OUT/idempotency-classification.json") runbook classes"
