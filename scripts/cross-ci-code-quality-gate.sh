#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/cross-ci-code-quality.json}"
OUT="${2:-results/generated/cross-ci-code-quality-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.cross-ci-code-quality/v1" and .minimum_public_repos >= 4 and (.required_artifacts | length) >= 5' "$SPEC" > /dev/null
grep -q 'reports:' examples/ci/gitlab-code-quality.yml
grep -q 'codequality: results/patchline/ci/gl-code-quality-report.json' examples/ci/gitlab-code-quality.yml
grep -q 'bitbucket-code-insights.json' examples/ci/bitbucket-pipelines.yml
grep -q 'analysis-bundle/summary.sarif' examples/ci/bitbucket-pipelines.yml

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --ci --out "$case_out/analyze" --json > "$case_out/analyze.json"
  test -s "$case_out/analyze/analysis-bundle/summary.sarif"
  test -s "$case_out/analyze/ci/gl-code-quality-report.json"
  test -s "$case_out/analyze/ci/bitbucket-code-insights.json"
  test -s "$case_out/analyze/ci/gitlab-ci-snippet.yml"
  test -s "$case_out/analyze/ci/bitbucket-pipelines-snippet.yml"
  jq -e 'type == "array" and length > 0 and all(.[]; .description and .check_name and .fingerprint and .severity and .location.path)' "$case_out/analyze/ci/gl-code-quality-report.json" > /dev/null
  jq -e '.version == "patchline.bitbucket-code-insights/v1" and (.annotations | length) > 0 and (.result == "PASSED" or .result == "FAILED") and all(.annotations[]; .external_id and .title and .severity and .path)' "$case_out/analyze/ci/bitbucket-code-insights.json" > /dev/null
  jq -e '.runs[0].results | length > 0' "$case_out/analyze/analysis-bundle/summary.sarif" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --slurpfile gitlab "$case_out/analyze/ci/gl-code-quality-report.json" \
    --slurpfile bitbucket "$case_out/analyze/ci/bitbucket-code-insights.json" \
    --slurpfile sarif "$case_out/analyze/analysis-bundle/summary.sarif" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      kind:"repo-slice",
      gitlab_findings:($gitlab[0] | length),
      bitbucket_annotations:($bitbucket[0].annotations | length),
      sarif_results:($sarif[0].runs[0].results | length),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.cross-ci-code-quality-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      gitlab_findings:($rows[0] | map(.gitlab_findings // 0) | add),
      bitbucket_annotations:($rows[0] | map(.bitbucket_annotations // 0) | add),
      sarif_results:($rows[0] | map(.sarif_results // 0) | add)
    }
  }' > "$OUT/cross-ci-code-quality.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.gitlab_findings >= (.slices | length) and
  .summary.bitbucket_annotations >= (.slices | length) and
  .summary.sarif_results >= (.slices | length)
' "$OUT/cross-ci-code-quality.json" > /dev/null

echo "cross-CI code-quality gate passed: $(jq '.summary.public_repos' "$OUT/cross-ci-code-quality.json") public repos, GitLab findings=$(jq '.summary.gitlab_findings' "$OUT/cross-ci-code-quality.json"), Bitbucket annotations=$(jq '.summary.bitbucket_annotations' "$OUT/cross-ci-code-quality.json"), SARIF results=$(jq '.summary.sarif_results' "$OUT/cross-ci-code-quality.json")"
