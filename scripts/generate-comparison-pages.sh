#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/comparison-pages-gate.json}"
OUT="${2:-results/generated/comparison-pages}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analyses" "$OUT/pages"

jq -e '
  .version == "patchline.comparison-pages-gate/v1" and
  (.claim | length) > 180 and
  (.real_code | length) >= .minimum_public_repos and
  (.pages | length) >= .minimum_pages and
  all(.real_code[]; (.id | length) > 0 and (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0) and
  all(.pages[]; (.id | test("^[a-z0-9-]+$")) and (.title | length) > 10 and (.category | length) > 0 and (.adjacent_strength | length) > 40 and (.patchline_difference | length) > 40 and (.complementary_workflow | length) > 40)
' "$SPEC" > /dev/null

analysis_rows=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  ecosystem="$(jq -r ".real_code[$i].ecosystem" "$SPEC")"
  framework="$(jq -r ".real_code[$i].framework" "$SPEC")"
  analysis="$OUT/analyses/$id"
  mkdir -p "$analysis"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=6,lines=90,tokens=10000,changes=2 \
    --no-llm \
    --ci \
    --out "$analysis" \
    --json > "$analysis/stdout.json"
  sarif_results="$(jq '[.runs[].results[]?] | length' "$analysis/analysis-bundle/summary.sarif")"
  gitlab_findings="$(jq 'length' "$analysis/ci/gl-code-quality-report.json")"
  bitbucket_annotations="$(jq '.annotations | length' "$analysis/ci/bitbucket-code-insights.json")"
  bundle_files="$(find "$analysis/analysis-bundle" -maxdepth 1 -type f | wc -l | tr -d ' ')"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" \
    --arg framework "$framework" \
    --argjson sarif_results "$sarif_results" \
    --argjson gitlab_findings "$gitlab_findings" \
    --argjson bitbucket_annotations "$bitbucket_annotations" \
    --argjson bundle_files "$bundle_files" \
    --slurpfile analyze "$analysis/analyze.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      ecosystem:$ecosystem,
      framework:$framework,
      files_scanned:$analyze[0].summary.files_scanned,
      ranked_risks:$analyze[0].summary.ranked_risks,
      provenance_slices:$analyze[0].summary.provenance_slices,
      generated_files:$analyze[0].summary.generated_files,
      compare_checks_failed:$analyze[0].summary.compare_checks_failed,
      deterministic_only:$analyze[0].summary.deterministic_only,
      sarif_results:$sarif_results,
      gitlab_code_quality_findings:$gitlab_findings,
      bitbucket_annotations:$bitbucket_annotations,
      analysis_bundle_files:$bundle_files,
      verified:true
    }' > "$analysis/comparison-row.json"
  analysis_rows+=("$analysis/comparison-row.json")
done

jq -s \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.comparison-evidence/v1",
    claim:$spec[0].claim,
    public_repos:length,
    analyses:.,
    summary:{
      public_repos:length,
      files_scanned:([.[].files_scanned] | add),
      ranked_risks:([.[].ranked_risks] | add),
      provenance_slices:([.[].provenance_slices] | add),
      generated_files:([.[].generated_files] | add),
      compare_checks_failed:([.[].compare_checks_failed] | add),
      sarif_results:([.[].sarif_results] | add),
      gitlab_code_quality_findings:([.[].gitlab_code_quality_findings] | add),
      bitbucket_annotations:([.[].bitbucket_annotations] | add),
      analysis_bundle_files:([.[].analysis_bundle_files] | add),
      deterministic_only:all(.[]; .deterministic_only == true)
    }
  }' "${analysis_rows[@]}" > "$OUT/evidence.json"

files="$(jq '.summary.files_scanned' "$OUT/evidence.json")"
risks="$(jq '.summary.ranked_risks' "$OUT/evidence.json")"
provenance="$(jq '.summary.provenance_slices' "$OUT/evidence.json")"
generated="$(jq '.summary.generated_files' "$OUT/evidence.json")"
failed="$(jq '.summary.compare_checks_failed' "$OUT/evidence.json")"
sarif="$(jq '.summary.sarif_results' "$OUT/evidence.json")"
gitlab="$(jq '.summary.gitlab_code_quality_findings' "$OUT/evidence.json")"
bitbucket="$(jq '.summary.bitbucket_annotations' "$OUT/evidence.json")"
bundle_files="$(jq '.summary.analysis_bundle_files' "$OUT/evidence.json")"

page_rows=()
page_count="$(jq '.pages | length' "$SPEC")"
for ((i=0; i<page_count; i++)); do
  id="$(jq -r ".pages[$i].id" "$SPEC")"
  title="$(jq -r ".pages[$i].title" "$SPEC")"
  category="$(jq -r ".pages[$i].category" "$SPEC")"
  strength="$(jq -r ".pages[$i].adjacent_strength" "$SPEC")"
  difference="$(jq -r ".pages[$i].patchline_difference" "$SPEC")"
  workflow="$(jq -r ".pages[$i].complementary_workflow" "$SPEC")"
  page="$OUT/pages/$id.md"
  cat > "$page" <<EOF
# $title

This comparison page is generated from pinned public-code evidence. It is not a claim that Patchline replaces $category; it explains where Patchline fits next to it.

## Adjacent tool strength

$strength

## Patchline difference

$difference

## Complementary workflow

$workflow

## Regenerated evidence

| Metric | Value |
| --- | ---: |
| public repositories | $count |
| files scanned | $files |
| ranked data-change risks | $risks |
| provenance slices | $provenance |
| generated review artifacts | $generated |
| deterministic compare checks failed | $failed |
| SARIF results | $sarif |
| GitLab code-quality findings | $gitlab |
| Bitbucket annotations | $bitbucket |
| analysis-bundle files | $bundle_files |

## Public slices

$(jq -r '.analyses[] | "- `" + .repo + "` `" + .subpath + "` at `" + .ref + "` (" + .ecosystem + " / " + .framework + ")"' "$OUT/evidence.json")
EOF
  jq -n \
    --arg id "$id" \
    --arg title "$title" \
    --arg category "$category" \
    --arg path "pages/$id.md" \
    --arg adjacent_strength "$strength" \
    --arg patchline_difference "$difference" \
    --arg complementary_workflow "$workflow" \
    '{
      id:$id,
      title:$title,
      category:$category,
      path:$path,
      adjacent_strength:$adjacent_strength,
      patchline_difference:$patchline_difference,
      complementary_workflow:$complementary_workflow,
      verified:true
    }' > "$OUT/pages/$id.json"
  page_rows+=("$OUT/pages/$id.json")
done

jq -s \
  --slurpfile evidence "$OUT/evidence.json" \
  '{
    version:"patchline.comparison-pages/v1",
    evidence:$evidence[0].summary,
    pages:.,
    summary:{
      pages:length,
      public_repos:$evidence[0].summary.public_repos,
      ranked_risks:$evidence[0].summary.ranked_risks,
      generated_files:$evidence[0].summary.generated_files,
      sarif_results:$evidence[0].summary.sarif_results,
      deterministic_only:$evidence[0].summary.deterministic_only
    }
  }' "${page_rows[@]}" > "$OUT/comparison-pages.json"

{
  echo "# Patchline comparisons"
  echo
  echo "Generated comparison pages against adjacent tools, backed by pinned public-code evidence."
  echo
  echo "| Page | Adjacent category | Best use |"
  echo "| --- | --- | --- |"
  jq -r '.pages[] | "| [" + .title + "](" + .path + ") | " + .category + " | " + .complementary_workflow + " |"' "$OUT/comparison-pages.json"
  echo
  echo "## Shared regenerated evidence"
  echo
  jq -r '.summary | "- public repositories: `" + (.public_repos|tostring) + "`\n- ranked data-change risks: `" + (.ranked_risks|tostring) + "`\n- generated review artifacts: `" + (.generated_files|tostring) + "`\n- SARIF results: `" + (.sarif_results|tostring) + "`\n- deterministic only: `" + (.deterministic_only|tostring) + "`"' "$OUT/comparison-pages.json"
} > "$OUT/index.md"

echo "comparison pages generated: $(jq '.summary.pages' "$OUT/comparison-pages.json") pages, risks $risks, SARIF $sarif"
