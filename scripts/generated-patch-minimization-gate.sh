#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/generated-patch-minimization.json}"
OUT="${2:-results/generated/generated-patch-minimization-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.generated-patch-minimization/v1" and .minimum_public_slices >= 4 and (.claim | contains("preserving risk coverage"))' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind all --budget files=5,lines=160,tokens=12000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  cp -R "$case_out/analyze/proposal" "$case_out/noisy-proposal"
  first_path="$(jq -r '.generated_files[0].path' "$case_out/noisy-proposal/proposal.json")"
  first_kind="$(jq -r '.generated_files[0].kind' "$case_out/noisy-proposal/proposal.json")"
  duplicate_path="patchline-proposals/minimizer/duplicate-$(basename "$first_path")"
  orphan_path="patchline-proposals/minimizer/orphan-no-coverage.md"
  mkdir -p "$case_out/noisy-proposal/$(dirname "$duplicate_path")"
  cp "$case_out/noisy-proposal/$first_path" "$case_out/noisy-proposal/$duplicate_path"
  printf '# untrusted generated orphan\n\nSuggested assertions:\n\n- should be removed because it targets no baseline risk.\n' > "$case_out/noisy-proposal/$orphan_path"
  tmp_json="$case_out/noisy-proposal/proposal.tmp.json"
  jq --arg duplicate_path "$duplicate_path" --arg duplicate_kind "$first_kind" --arg orphan_path "$orphan_path" '
    .generated_files += [
      {path:$duplicate_path, kind:$duplicate_kind, content_hash:"sha256:duplicate", risk_ids:.generated_files[0].risk_ids},
      {path:$orphan_path, kind:"tests", content_hash:"sha256:orphan", risk_ids:["risk:not-in-baseline"]}
    ]
  ' "$case_out/noisy-proposal/proposal.json" > "$tmp_json"
  mv "$tmp_json" "$case_out/noisy-proposal/proposal.json"

  go run ./cmd/patchline repo proposal-minimize --before "$case_out/analyze/baseline" --after "$case_out/noisy-proposal" --out "$case_out/minimized-proposal" --json > "$case_out/minimized.json"
  go run ./cmd/patchline repo compare --before "$case_out/analyze/baseline" --after "$case_out/minimized-proposal" --out "$case_out/minimized-compare" --json > "$case_out/minimized-compare.json"

  original_coverage="$(jq '.summary.risks_with_coverage' "$case_out/analyze/compare/compare.json")"
  jq -e --argjson original_coverage "$original_coverage" '
    .minimization.applied == true and
    .minimization.before_files > .minimization.after_files and
    .minimization.removed_files >= 2 and
    any(.minimization.removed[]; .reason == "no-target-risk-coverage") and
    any(.minimization.removed[]; .reason == "no-new-risk-coverage" or .reason == "duplicate-generated-hunk")
  ' "$case_out/minimized.json" > /dev/null
  jq -e --argjson original_coverage "$original_coverage" '
    .summary.patchline_checks_failed == 0 and
    .summary.risks_with_coverage == $original_coverage
  ' "$case_out/minimized-compare.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --slurpfile minimized "$case_out/minimized.json" \
    --slurpfile compare "$case_out/minimized-compare.json" \
    '{id:$id, repo:$repo, before_files:$minimized[0].minimization.before_files, after_files:$minimized[0].minimization.after_files, removed_files:$minimized[0].minimization.removed_files, risks_with_coverage:$compare[0].summary.risks_with_coverage, patchline_checks_failed:$compare[0].summary.patchline_checks_failed, verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.generated-patch-minimization-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      removed_files:($rows[0] | map(.removed_files) | add)
    }
  }' > "$OUT/generated-patch-minimization.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.verified == (.slices | length) and
  .summary.removed_files >= (.slices | length * 2) and
  all(.slices[]; .verified == true and .after_files < .before_files and .patchline_checks_failed == 0 and .risks_with_coverage > 0)
' "$OUT/generated-patch-minimization.json" > /dev/null

echo "generated patch minimization gate passed: $(jq '.summary.removed_files' "$OUT/generated-patch-minimization.json") redundant files removed"
