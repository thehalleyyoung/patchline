#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/fuzz-coverage-gate.json}"
OUT="${2:-results/generated/fuzz-coverage-gate}"
DOC="${3:-docs/fuzzing.md}"
rm -rf "$OUT"
mkdir -p "$OUT/cases" "$OUT/cache" "$OUT/fuzz"

jq -e '
  .version == "patchline.fuzz-coverage-gate/v1" and
  (.fuzztime | length) > 0 and
  .minimum_public_repos >= 4 and
  (.required_targets | length) >= 7 and
  (.slices | length) >= .minimum_public_repos and
  all(.required_targets[]; (.package | length) > 0 and (.name | startswith("Fuzz")) and (.area | length) > 0) and
  all(.slices[]; (.id | length) > 0 and (.repo | length) > 0 and (.ref | length) == 40 and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for term in \
  "FuzzAnalyzeBytesWithDialect" \
  "FuzzExtractSourceSQL" \
  "FuzzFactNormalization" \
  "FuzzArchiveExtraction" \
  "FuzzReportLoading" \
  "FuzzBundleRedactor" \
  "make fuzz-coverage-gate"; do
  grep -F "$term" "$DOC" > /dev/null
done

while IFS=$'\t' read -r pkg name area; do
  rg -n "func ${name}\\(" "${pkg#./}" > "$OUT/fuzz/${name}.rg"
done < <(jq -r '.required_targets[] | [.package, .name, .area] | @tsv' "$SPEC")

go test ./internal/migration ./internal/project ./cmd/patchline

fuzztime="$(jq -r '.fuzztime' "$SPEC")"
target_rows=()
while IFS=$'\t' read -r pkg name area; do
  log="$OUT/fuzz/${name}.log"
  go test "$pkg" -run '^$' -fuzz="$name" -fuzztime="$fuzztime" -parallel=1 > "$log"
  grep -F "PASS" "$log" > /dev/null
  jq -n \
    --arg package "$pkg" \
    --arg name "$name" \
    --arg area "$area" \
    --arg log "$log" \
    '{package:$package,name:$name,area:$area,log:$log,verified:true}' > "$OUT/fuzz/${name}.json"
  target_rows+=("$OUT/fuzz/${name}.json")
done < <(jq -r '.required_targets[] | [.package, .name, .area] | @tsv' "$SPEC")

slice_rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline golden-fixture generate \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --out "$case_out/golden" \
    --max-files 3 \
    --max-file-bytes 24576 \
    --max-total-bytes 49152 \
    --json > "$case_out/stdout.json"
  (cd "$case_out/golden" && go test .) > "$case_out/generated-go-test.log"
  jq -e '
    .version == "patchline.golden-fixture/v1" and
    .summary.original_files_scanned > .summary.selected_files and
    .expectations.ranked_risks > 0 and
    .expectations.policy_checks > 0 and
    (.outputs.test | length) > 0
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
      ranked_risks:$fixture[0].expectations.ranked_risks,
      verified:true
    }' > "$case_out/row.json"
  slice_rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile targets <(jq -s '.' "${target_rows[@]}") \
  --slurpfile slices <(jq -s '.' "${slice_rows[@]}") \
  '{
    version:"patchline.fuzz-coverage-gate-results/v1",
    claim:$spec[0].claim,
    fuzztime:$spec[0].fuzztime,
    targets:$targets[0],
    slices:$slices[0],
    summary:{
      fuzz_targets:($targets[0] | length),
      fuzz_targets_verified:($targets[0] | map(select(.verified == true)) | length),
      public_repos:($slices[0] | map(.repo) | unique | length),
      real_repo_slices_verified:($slices[0] | map(select(.verified == true)) | length),
      original_files:($slices[0] | map(.original_files) | add),
      selected_files:($slices[0] | map(.selected_files) | add),
      ranked_risks:($slices[0] | map(.ranked_risks) | add)
    }
  }' > "$OUT/summary.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.fuzz-coverage-gate-results/v1" and
  .summary.fuzz_targets >= ($spec[0].required_targets | length) and
  .summary.fuzz_targets_verified == .summary.fuzz_targets and
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.real_repo_slices_verified >= $spec[0].minimum_public_repos and
  .summary.original_files > .summary.selected_files and
  .summary.ranked_risks > 0
' "$OUT/summary.json" > /dev/null

echo "fuzz coverage gate passed: $(jq '.summary.fuzz_targets' "$OUT/summary.json") fuzz targets and $(jq '.summary.public_repos' "$OUT/summary.json") public repo slices verified"
