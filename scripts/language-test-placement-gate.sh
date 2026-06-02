#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/language-test-placement.json}"
OUT="${2:-results/generated/language-test-placement-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.language-test-placement/v1" and
  (.cases | length) >= 6 and
  ([.cases[].id] | index("rails")) and
  ([.cases[].id] | index("django")) and
  ([.cases[].id] | index("go")) and
  ([.cases[].id] | index("java")) and
  ([.cases[].id] | index("node")) and
  ([.cases[].id] | index("python"))
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath expected_prefix expected_suffix; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind tests --budget files=1,lines=120,tokens=4000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  path="$(jq -r '.generated_files[0].path' "$case_out/analyze/proposal/proposal.json")"
  test -n "$path"
  case "$path" in
    "$expected_prefix"*"$expected_suffix") ;;
    *)
      echo "$id generated unexpected test path: $path" >&2
      exit 1
      ;;
  esac
  jq -e --arg path "$path" '.summary.generated_files == 1 and .summary.patchline_checks_failed == 0 and any(.generated_checks[]; .path == $path and (.status == "pass" or .status == "warn"))' "$case_out/analyze/compare/compare.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg path "$path" \
    --arg expected_prefix "$expected_prefix" \
    --arg expected_suffix "$expected_suffix" \
    '{id:$id, repo:$repo, subpath:$subpath, generated_test_path:$path, expected_prefix:$expected_prefix, expected_suffix:$expected_suffix, verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.cases[] | [.id, .repo, .ref, .subpath, .expected_prefix, .expected_suffix] | @tsv' "$SPEC")

jq -s '{version:"patchline.language-test-placement-results/v1", cases:.}' "${rows[@]}" > "$OUT/language-test-placement.json"
jq -e '(.cases | length) >= 6 and all(.cases[]; .verified == true and (.generated_test_path | length) > 0)' "$OUT/language-test-placement.json" > /dev/null

echo "language test placement gate passed: $(jq '.cases | length' "$OUT/language-test-placement.json") ecosystem placements verified"
