#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/golden-fixture-gate.json}"
OUT="${2:-results/generated/golden-fixture-gate}"
DOC="${3:-docs/golden-fixtures.md}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/cache"

jq -e '
  .version == "patchline.golden-fixture-gate/v1" and
  .minimum_public_repos >= 4 and
  (.slices | length) >= .minimum_public_repos and
  .max_files >= 1 and .max_files <= 5 and
  .max_file_bytes > 0 and .max_total_bytes > 0 and
  .minimum_reduction_percent >= 50 and
  all(.slices[]; (.id | length) > 0 and (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for term in \
  "golden-fixture generate" \
  "generated_golden_test.go" \
  "without vendoring" \
  "go test" \
  "golden-fixture.json"; do
  grep -F "$term" "$DOC" > /dev/null
done

go test ./internal/goldenfixture ./cmd/patchline

rows=()
while IFS=$'\t' read -r id repo ref subpath max_files max_file_bytes max_total_bytes min_reduction; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline golden-fixture generate \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --out "$case_out/golden" \
    --max-files "$max_files" \
    --max-file-bytes "$max_file_bytes" \
    --max-total-bytes "$max_total_bytes" \
    --json > "$case_out/stdout.json"

  test -s "$case_out/golden/generated_golden_test.go"
  test -s "$case_out/golden/go.mod"
  test -s "$case_out/golden/golden-fixture.json"
  test -s "$case_out/golden/golden-fixture.md"
  (cd "$case_out/golden" && go test .) > "$case_out/go-test.log"

  jq -e \
    --argjson max_files "$max_files" \
    --argjson max_total_bytes "$max_total_bytes" \
    --argjson min_reduction "$min_reduction" \
    '
      .version == "patchline.golden-fixture/v1" and
      (.hash | length) > 0 and
      .summary.original_files_scanned > .summary.selected_files and
      .summary.original_ranked_risks > 0 and
      .summary.selected_files > 0 and .summary.selected_files <= $max_files and
      .summary.selected_bytes > 0 and .summary.selected_bytes <= $max_total_bytes and
      .summary.reduction_percent >= $min_reduction and
      .expectations.files_scanned == .summary.selected_files and
      .expectations.ranked_risks > 0 and
      .expectations.policy_checks > 0 and
      (.selected_files | length) == .summary.selected_files and
      all(.selected_files[]; (.path | length) > 0 and (.content_hash | length) > 0 and .bytes > 0)
    ' "$case_out/golden/golden-fixture.json" > /dev/null

  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --slurpfile fixture "$case_out/golden/golden-fixture.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      original_files:$fixture[0].summary.original_files_scanned,
      selected_files:$fixture[0].summary.selected_files,
      selected_bytes:$fixture[0].summary.selected_bytes,
      reduction_percent:$fixture[0].summary.reduction_percent,
      ranked_risks:$fixture[0].expectations.ranked_risks,
      generated_test_bytes:$fixture[0].summary.generated_test_bytes,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '[.max_files, .max_file_bytes, .max_total_bytes, .minimum_reduction_percent] as $limits | .slices[] | [.id, .repo, .ref, .subpath, $limits[]] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.golden-fixture-gate-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      original_files:($rows[0] | map(.original_files) | add),
      selected_files:($rows[0] | map(.selected_files) | add),
      selected_bytes:($rows[0] | map(.selected_bytes) | add),
      generated_test_bytes:($rows[0] | map(.generated_test_bytes) | add),
      ranked_risks:($rows[0] | map(.ranked_risks) | add),
      min_reduction_percent:($rows[0] | map(.reduction_percent) | min)
    }
  }' > "$OUT/summary.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.golden-fixture-gate-results/v1" and
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified >= $spec[0].minimum_public_repos and
  .summary.original_files > .summary.selected_files and
  .summary.selected_files <= ($spec[0].max_files * $spec[0].minimum_public_repos) and
  .summary.ranked_risks > 0 and
  .summary.min_reduction_percent >= $spec[0].minimum_reduction_percent
' "$OUT/summary.json" > /dev/null

echo "golden fixture gate passed: $(jq '.summary.public_repos' "$OUT/summary.json") public repo slices generated and executed minimal tests"
