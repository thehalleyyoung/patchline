#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/public-repo-many-matrix.json}"
OUT="${2:-results/generated/public-repo-many-matrix}"
BIN="$OUT/patchline"

jq -e '.version == "patchline.public-repo-many-matrix/v1" and (.cases | length) >= 10' "$SPEC" > /dev/null

rm -rf "$OUT"
mkdir -p "$OUT/cases"
go build -o "$BIN" ./cmd/patchline

rows=()
while IFS=$'\t' read -r id repo subpath ecosystem evidence; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  echo "==> $id $repo:$subpath"
  "$BIN" repo analyze --github "$repo" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=3,lines=80,tokens=8000,changes=2 \
    --no-llm \
    --out "$case_out/analyze" \
    --json > "$case_out/analyze.json"
  "$BIN" repo offline \
    --analysis "$case_out/analyze" \
    --out "$case_out/offline" \
    --json > "$case_out/offline.json"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --arg evidence "$evidence" \
    --slurpfile analyze "$case_out/analyze/analyze.json" \
    --slurpfile source "$case_out/analyze/fetch/source.json" \
    --slurpfile offline "$case_out/offline.json" \
    '{
      id:$id,
      repo:$repo,
      subpath:$subpath,
      ecosystem:$ecosystem,
      evidence:$evidence,
      resolved_commit:$source[0].resolved_commit,
      files_scanned:$analyze[0].summary.files_scanned,
      facts:$analyze[0].summary.facts,
      ranked_risks:$analyze[0].summary.ranked_risks,
      generated_files:$analyze[0].summary.generated_files,
      compare_checks_failed:$analyze[0].summary.compare_checks_failed,
      deterministic_only:$analyze[0].summary.deterministic_only,
      offline_ok:$offline[0].ok
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.cases[] | [.id,.repo,.subpath,.ecosystem,.evidence] | @tsv' "$SPEC")

jq -s '
  {
    version:"patchline.public-repo-many-matrix-results/v1",
    summary:{
      slices:length,
      unique_repos:(map(.repo) | unique | length),
      ecosystems:(map(.ecosystem) | unique | length),
      total_files_scanned:(map(.files_scanned) | add),
      total_ranked_risks:(map(.ranked_risks) | add),
      total_generated_files:(map(.generated_files) | add),
      offline_passed:(map(select(.offline_ok == true)) | length),
      all_compare_checks_passed: all(.compare_checks_failed == 0),
      all_deterministic: all(.deterministic_only == true)
    },
    cases:.
  }
' "${rows[@]}" > "$OUT/summary.json"

jq -e '
  .version == "patchline.public-repo-many-matrix-results/v1" and
  .summary.slices >= 10 and
  .summary.unique_repos >= 10 and
  .summary.ecosystems >= 10 and
  .summary.total_files_scanned > 0 and
  .summary.offline_passed == .summary.slices and
  .summary.all_compare_checks_passed == true and
  .summary.all_deterministic == true and
  all(.cases[]; (.resolved_commit | test("^[0-9a-f]{40}$")) and .files_scanned > 0 and .facts >= .files_scanned and .offline_ok == true)
' "$OUT/summary.json" > /dev/null

cp "$OUT/summary.json" examples/public-repo-many-matrix-results.json
echo "public repo many-matrix gate passed: $(jq '.summary.slices' "$OUT/summary.json") slices across $(jq '.summary.unique_repos' "$OUT/summary.json") repos"
