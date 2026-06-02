#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/transaction-boundary-inference.json}"
OUT="${2:-results/generated/transaction-boundary-inference-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.transaction-boundary-inference/v1" and .minimum_public_slices >= 4 and (.required_surfaces | index("baseline")) != null and (.required_surfaces | index("generated_repair")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind all --budget files=6,lines=160,tokens=16000,changes=3 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.transaction_boundaries > 0 and
    (.transaction_boundaries | length) == .summary.transaction_boundaries and
    any(.transaction_boundaries[]; (.status == "missing" or .status == "partial" or .status == "explicit") and (.surface | length) > 0)
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -e '
    .summary.transaction_boundaries > 0 and
    .summary.transaction_explicit > 0 and
    any(.transaction_boundaries[]; .surface == "generated_repair" and .status == "explicit")
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
      baseline_boundaries:$baseline[0].summary.transaction_boundaries,
      baseline_missing:$baseline[0].summary.transaction_missing,
      baseline_partial:$baseline[0].summary.transaction_partial,
      baseline_explicit:$baseline[0].summary.transaction_explicit,
      generated_boundaries:$compare[0].summary.transaction_boundaries,
      generated_explicit:$compare[0].summary.transaction_explicit,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.transaction-boundary-inference-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      baseline_boundaries:($rows[0] | map(.baseline_boundaries) | add),
      generated_boundaries:($rows[0] | map(.generated_boundaries) | add)
    }
  }' > "$OUT/transaction-boundary-inference.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.verified == (.slices | length) and
  .summary.baseline_boundaries >= (.slices | length) and
  .summary.generated_boundaries >= (.slices | length)
' "$OUT/transaction-boundary-inference.json" > /dev/null

echo "transaction boundary inference gate passed: $(jq '.summary.verified' "$OUT/transaction-boundary-inference.json") public slices, $(jq '.summary.baseline_boundaries' "$OUT/transaction-boundary-inference.json") baseline boundaries, $(jq '.summary.generated_boundaries' "$OUT/transaction-boundary-inference.json") generated boundaries"
