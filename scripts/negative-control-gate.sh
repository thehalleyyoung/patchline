#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/negative-control-slices.json}"
OUT="${2:-results/generated/negative-control-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.negative-control-slices/v1" and
  (.claim | contains("high-confidence repair claims")) and
  (.slices | length) >= 4 and
  ([.slices[].category] | index("documentation-only")) and
  ([.slices[].category] | index("vendor-only")) and
  ([.slices[].category] | index("test-only")) and
  all(.slices[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id category repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/fetch" --json > "$case_out/fetch.json"
  scan_root="$(jq -r '.source.scanned_root' "$case_out/fetch.json")"
  go run ./cmd/patchline intake "$scan_root" --out "$case_out/intake" --json > "$case_out/intake.json"
  go run ./cmd/patchline repo analyze "$scan_root" --stages inventory,baseline --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -n \
    --arg id "$id" \
    --arg category "$category" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile fetch "$case_out/fetch.json" \
    --slurpfile intake "$case_out/intake.json" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    '{
      id:$id,
      category:$category,
      repo:$repo,
      subpath:$subpath,
      resolved_commit:$fetch[0].source.resolved_commit,
      files_scanned:$intake[0].summary.files_scanned,
      repair_candidates:$intake[0].summary.repair_candidates,
      high_confidence_repair_candidates:([$intake[0].repair_candidates[]? | select(.confidence == "high")] | length),
      checked_repair_proofs:($baseline[0].summary.repair_proof_checked // 0),
      conditional_repair_proofs:($baseline[0].summary.repair_proof_conditional // 0),
      open_repair_proofs:($baseline[0].summary.repair_proof_open // 0),
      refuted_repair_proofs:($baseline[0].summary.repair_proof_refuted // 0),
      ranked_risks:($baseline[0].summary.ranked_risks // 0),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .category, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.negative-control-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      categories:($rows[0] | map(.category) | unique),
      high_confidence_repair_candidates:($rows[0] | map(.high_confidence_repair_candidates) | add),
      checked_repair_proofs:($rows[0] | map(.checked_repair_proofs) | add)
    }
  }' > "$OUT/negative-controls.json"

jq -e '
  .version == "patchline.negative-control-results/v1" and
  (.slices | length) >= 4 and
  (.summary.categories | index("documentation-only")) and
  (.summary.categories | index("vendor-only")) and
  (.summary.categories | index("test-only")) and
  .summary.high_confidence_repair_candidates == 0 and
  .summary.checked_repair_proofs == 0 and
  all(.slices[]; .verified == true and .files_scanned > 0 and .high_confidence_repair_candidates == 0 and .checked_repair_proofs == 0)
' "$OUT/negative-controls.json" > /dev/null

echo "negative control gate passed: $(jq '.summary.public_slices' "$OUT/negative-controls.json") public slices avoided high-confidence repair claims"
