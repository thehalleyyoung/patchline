#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/guard-mutation.json}"
OUT="${2:-results/generated/guard-mutation-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.guard-mutation/v1" and
  .minimum_public_slices >= 4 and
  ([.mutations[].id] | index("delete-table-existence")) and
  ([.mutations[].id] | index("delete-row-count")) and
  ([.mutations[].id] | index("delete-rollback")) and
  (.claim | contains("deterministic compare"))
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind guards --budget files=1,lines=120,tokens=4000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  guard_path="$(jq -r '.generated_files[0].path' "$case_out/analyze/proposal/proposal.json")"
  test -n "$guard_path"
  jq -e --arg path "$guard_path" '.summary.patchline_checks_failed == 0 and any(.generated_checks[]; .path == $path and .status == "pass")' "$case_out/analyze/compare/compare.json" > /dev/null

  mutation_rows=()
  while IFS=$'\t' read -r mutation remove; do
    mutation_out="$case_out/mutations/$mutation"
    mkdir -p "$mutation_out/proposal"
    cp "$case_out/analyze/proposal/proposal.json" "$mutation_out/proposal/proposal.json"
    mkdir -p "$mutation_out/proposal/$(dirname "$guard_path")"
    case "$mutation" in
      delete-table-existence)
        sed 's/SELECT 1 FROM/MUTATED_REQUIRED_CHECK/g' "$case_out/analyze/proposal/$guard_path" > "$mutation_out/proposal/$guard_path"
        ;;
      delete-row-count)
        sed 's/count(\*)/MUTATED_REQUIRED_CHECK/g' "$case_out/analyze/proposal/$guard_path" > "$mutation_out/proposal/$guard_path"
        ;;
      delete-rollback)
        sed 's/ROLLBACK/MUTATED_REQUIRED_CHECK/g' "$case_out/analyze/proposal/$guard_path" > "$mutation_out/proposal/$guard_path"
        ;;
      *)
        echo "unknown mutation: $mutation" >&2
        exit 1
        ;;
    esac
    go run ./cmd/patchline repo compare --before "$case_out/analyze/baseline" --after "$mutation_out/proposal" --out "$mutation_out/compare" --json > "$mutation_out/compare.json"
    jq -e --arg path "$guard_path" '.summary.patchline_checks_failed > 0 and any(.generated_checks[]; .path == $path and .status == "fail")' "$mutation_out/compare.json" > /dev/null
    jq -n \
      --arg id "$mutation" \
      --arg removed "$remove" \
      --slurpfile compare "$mutation_out/compare.json" \
      '{id:$id, removed:$removed, failed_checks:$compare[0].summary.patchline_checks_failed, findings:($compare[0].generated_checks[0].findings // []), verified:true}' > "$mutation_out/row.json"
    mutation_rows+=("$mutation_out/row.json")
  done < <(jq -r '.mutations[] | [.id, .remove] | @tsv' "$SPEC")

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg guard_path "$guard_path" \
    --slurpfile mutations <(jq -s '.' "${mutation_rows[@]}") \
    '{id:$id, repo:$repo, subpath:$subpath, guard_path:$guard_path, mutations:$mutations[0], verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.guard-mutation-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      mutations:($rows[0] | map(.mutations | length) | add),
      rejected_mutations:($rows[0] | map(.mutations[] | select(.verified == true)) | length)
    }
  }' > "$OUT/guard-mutations.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.guard-mutation-results/v1" and
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.mutations == (.summary.public_slices * ($spec[0].mutations | length)) and
  .summary.rejected_mutations == .summary.mutations and
  all(.slices[]; .verified == true and (.mutations | length) == ($spec[0].mutations | length) and all(.mutations[]; .verified == true and .failed_checks > 0))
' "$OUT/guard-mutations.json" > /dev/null

echo "guard mutation gate passed: $(jq '.summary.rejected_mutations' "$OUT/guard-mutations.json") weakened guards rejected"
