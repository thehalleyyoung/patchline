#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/safe-review-badge.json}"
OUT="${2:-results/generated/safe-review-badge-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.safe-review-badge/v1" and .minimum_public_slices >= 4 and (.claim | contains("proof holes are listed"))' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind explain --budget files=1,lines=120,tokens=4000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .review_badge.safe == true and
    .review_badge.status == "safe-to-review" and
    (.review_badge.proof_holes | length) > 0 and
    .summary.patchline_checks_failed == 0 and
    .summary.risk_budget_rejected == false and
    (
      ((.native_checks // []) | length) == 0 or
      all((.native_results // [])[]; .status == "pass" or (.status == "skipped" and ((.skipped_reason // "") | length) > 0))
    )
  ' "$case_out/analyze/compare/compare.json" > /dev/null

  cp -R "$case_out/analyze/proposal" "$case_out/risky-proposal"
  explain_path="$(jq -r '.generated_files[0].path' "$case_out/risky-proposal/proposal.json")"
  cat >> "$case_out/risky-proposal/$explain_path" <<'SQL'

-- Badge mutation: unsafe generated writes remove the safe-to-review badge.
UPDATE patchline_badge_probe SET unsafe = true;
DELETE FROM patchline_badge_probe;
SQL
  go run ./cmd/patchline repo compare --before "$case_out/analyze/baseline" --after "$case_out/risky-proposal" --out "$case_out/risky-compare" --json > "$case_out/risky-compare.json"
  jq -e '
    .review_badge.safe == false and
    .review_badge.status == "not-safe-to-review" and
    (.review_badge.proof_holes | length) > 0 and
    .summary.risk_budget_rejected == true
  ' "$case_out/risky-compare.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --slurpfile safe "$case_out/analyze/compare/compare.json" \
    --slurpfile risky "$case_out/risky-compare.json" \
    '{id:$id, repo:$repo, safe_status:$safe[0].review_badge.status, unsafe_status:$risky[0].review_badge.status, proof_holes:($safe[0].review_badge.proof_holes | length), verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.safe-review-badge-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      verified:($rows[0] | map(select(.verified == true)) | length)
    }
  }' > "$OUT/safe-review-badge.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.verified == (.slices | length) and
  all(.slices[]; .safe_status == "safe-to-review" and .unsafe_status == "not-safe-to-review" and .proof_holes > 0)
' "$OUT/safe-review-badge.json" > /dev/null

echo "safe review badge gate passed: $(jq '.summary.verified' "$OUT/safe-review-badge.json") public slices checked"
