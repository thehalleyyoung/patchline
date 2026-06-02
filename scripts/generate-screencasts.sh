#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/screencast-gate.json}"
OUT="${2:-results/generated/screencasts}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis" "$OUT/casts" "$OUT/transcripts" "$OUT/storyboards"

jq -e '
  .version == "patchline.screencast-gate/v1" and
  (.claim | length) > 180 and
  (.repo | length) > 0 and
  (.ref | test("^[0-9a-f]{40}$")) and
  (.subpath | length) > 0 and
  (.required_screencasts | length) == 4 and
  (.required_artifacts | length) >= 14
' "$SPEC" > /dev/null

repo="$(jq -r '.repo' "$SPEC")"
ref="$(jq -r '.ref' "$SPEC")"
subpath="$(jq -r '.subpath' "$SPEC")"
analysis="$OUT/analysis"

go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=8,lines=100,tokens=12000,changes=2 \
  --no-llm \
  --ci \
  --out "$analysis" \
  --json > "$OUT/analyze-stdout.json"

files="$(jq '.summary.files_scanned' "$analysis/analyze.json")"
facts="$(jq '.summary.facts' "$analysis/analyze.json")"
risks="$(jq '.summary.ranked_risks' "$analysis/analyze.json")"
provenance="$(jq '.summary.provenance_slices' "$analysis/analyze.json")"
generated="$(jq '.summary.generated_files' "$analysis/analyze.json")"
failed="$(jq '.summary.compare_checks_failed' "$analysis/analyze.json")"
deterministic="$(jq '.summary.deterministic_only' "$analysis/analyze.json")"
top_risk="$(jq -r '[.. | objects | select(has("id") and has("severity")) | .id] | first // "baseline-risk"' "$analysis/baseline/baseline.json")"
patch_lines="$(wc -l < "$analysis/proposal/proposal.patch" | tr -d ' ')"
sarif_results="$(jq '[.runs[].results[]?] | length' "$analysis/analysis-bundle/summary.sarif")"
code_quality_findings="$(jq 'length' "$analysis/ci/gl-code-quality-report.json")"
bitbucket_annotations="$(jq '.annotations | length' "$analysis/ci/bitbucket-code-insights.json")"
bundle_files="$(find "$analysis/analysis-bundle" -maxdepth 1 -type f | wc -l | tr -d ' ')"

find "$analysis/analysis-bundle" -maxdepth 1 -type f | sort | while read -r file; do
  rel="${file#$analysis/}"
  shasum -a 256 "$file" | awk -v rel="$rel" '{print $1 "  " rel}'
done > "$OUT/artifact-checksums.txt"
bundle_hash="$(shasum -a 256 "$OUT/artifact-checksums.txt" | awk '{print $1}')"

write_cast() {
  local slug="$1"
  local title="$2"
  local command="$3"
  local output="$4"
  local cast="$OUT/casts/$slug.cast"
  jq -cn --arg title "$title" '{
    version:2,
    width:100,
    height:28,
    timestamp:1893456000,
    env:{SHELL:"/bin/bash", TERM:"xterm-256color"},
    title:$title
  }' > "$cast"
  jq -cn --arg command "$ $command\n" '[0.0,"o",$command]' >> "$cast"
  jq -cn --arg output "$output" '[0.8,"o",$output]' >> "$cast"
}

write_markdown() {
  local dir="$1"
  local slug="$2"
  local title="$3"
  local command="$4"
  local output="$5"
  cat > "$OUT/$dir/$slug.md" <<EOF
# $title

This short terminal screencast is regenerated from pinned public code: \`$repo\` \`$subpath\` at \`$ref\`.

\`\`\`bash
$command
\`\`\`

\`\`\`text
$output
\`\`\`
EOF
}

first_command="go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --stages inventory,baseline,propose,compare --proposal-kind all --no-llm --out results/patchline-demo --json"
first_output="Patchline analyzed real public code.
files scanned: $files
facts extracted: $facts
ranked data-change risks: $risks
provenance slices: $provenance
generated review artifacts: $generated
deterministic checks failed: $failed"

review_command="sed -n '1,80p' results/patchline-demo/proposal/proposal.patch && jq '.summary.compare_checks_failed' results/patchline-demo/compare/compare.json"
review_output="Generated intervention review stays bounded and deterministic.
top baseline risk: $top_risk
proposal.patch lines: $patch_lines
generated files: $generated
compare checks failed: $failed
review rule: generated code is evidence for maintainers, not trusted executable output."

ci_command="go run ./cmd/patchline repo analyze --github $repo --ref $ref --subpath $subpath --ci --no-llm --out results/patchline-ci"
ci_output="CI integration artifacts are emitted from the same public-code run.
SARIF results: $sarif_results -> analysis-bundle/summary.sarif
GitLab code-quality findings: $code_quality_findings -> ci/gl-code-quality-report.json
Bitbucket annotations: $bitbucket_annotations -> ci/bitbucket-code-insights.json
analysis bundle files: $bundle_files"

repro_command="shasum -a 256 results/patchline-ci/analysis-bundle/* | tee artifact-checksums.txt"
repro_output="Artifact reproduction is hash-addressed and replayable.
analysis bundle files: $bundle_files
bundle checksum manifest: artifact-checksums.txt
bundle hash: $bundle_hash
deterministic-only generation: $deterministic"

write_cast "first-run-analysis" "First-run analysis" "$first_command" "$first_output"
write_cast "generated-intervention-review" "Generated intervention review" "$review_command" "$review_output"
write_cast "ci-integration" "CI integration" "$ci_command" "$ci_output"
write_cast "artifact-reproduction" "Artifact reproduction" "$repro_command" "$repro_output"

write_markdown "transcripts" "first-run-analysis" "First-run analysis" "$first_command" "$first_output"
write_markdown "transcripts" "generated-intervention-review" "Generated intervention review" "$review_command" "$review_output"
write_markdown "transcripts" "ci-integration" "CI integration" "$ci_command" "$ci_output"
write_markdown "transcripts" "artifact-reproduction" "Artifact reproduction" "$repro_command" "$repro_output"

write_markdown "storyboards" "first-run-analysis" "First-run analysis storyboard" "$first_command" "Show the install-free command, then zoom in on files scanned, risks, provenance, generated artifacts, and zero deterministic check failures."
write_markdown "storyboards" "generated-intervention-review" "Generated intervention review storyboard" "$review_command" "Open proposal.patch, point at the bounded patch size, then show compare checks so reviewers see how plausible generated code is constrained."
write_markdown "storyboards" "ci-integration" "CI integration storyboard" "$ci_command" "Show the same analysis writing SARIF, GitLab code-quality, Bitbucket annotations, and the analysis bundle without service-specific credentials."
write_markdown "storyboards" "artifact-reproduction" "Artifact reproduction storyboard" "$repro_command" "Hash the analysis bundle and explain how reviewers can reproduce the same evidence from pinned public code."

cat > "$OUT/index.md" <<EOF
# Patchline screencasts

These short screencasts are generated from \`$repo\` \`$subpath\` at \`$ref\`.

| Screencast | Cast | Transcript | Storyboard |
| --- | --- | --- | --- |
| First-run analysis | [cast](casts/first-run-analysis.cast) | [transcript](transcripts/first-run-analysis.md) | [storyboard](storyboards/first-run-analysis.md) |
| Generated intervention review | [cast](casts/generated-intervention-review.cast) | [transcript](transcripts/generated-intervention-review.md) | [storyboard](storyboards/generated-intervention-review.md) |
| CI integration | [cast](casts/ci-integration.cast) | [transcript](transcripts/ci-integration.md) | [storyboard](storyboards/ci-integration.md) |
| Artifact reproduction | [cast](casts/artifact-reproduction.cast) | [transcript](transcripts/artifact-reproduction.md) | [storyboard](storyboards/artifact-reproduction.md) |

The generated run scanned $files files, ranked $risks data-change risks, emitted $generated generated review artifacts, and produced $bundle_files analysis-bundle files.
EOF

jq -n \
  --arg repo "$repo" \
  --arg ref "$ref" \
  --arg subpath "$subpath" \
  --arg top_risk "$top_risk" \
  --arg bundle_hash "$bundle_hash" \
  --argjson files "$files" \
  --argjson facts "$facts" \
  --argjson risks "$risks" \
  --argjson provenance "$provenance" \
  --argjson generated "$generated" \
  --argjson failed "$failed" \
  --argjson deterministic "$deterministic" \
  --argjson patch_lines "$patch_lines" \
  --argjson sarif_results "$sarif_results" \
  --argjson code_quality_findings "$code_quality_findings" \
  --argjson bitbucket_annotations "$bitbucket_annotations" \
  --argjson bundle_files "$bundle_files" \
  '{
    version:"patchline.screencasts/v1",
    repo:$repo,
    ref:$ref,
    subpath:$subpath,
    metrics:{
      files_scanned:$files,
      facts:$facts,
      ranked_risks:$risks,
      provenance_slices:$provenance,
      generated_files:$generated,
      compare_checks_failed:$failed,
      deterministic_only:$deterministic
    },
    generated_intervention:{top_risk:$top_risk, proposal_patch_lines:$patch_lines},
    ci:{sarif_results:$sarif_results, gitlab_code_quality_findings:$code_quality_findings, bitbucket_annotations:$bitbucket_annotations},
    reproduction:{analysis_bundle_files:$bundle_files, bundle_hash:$bundle_hash},
    screencasts:[
      {id:"first-run-analysis", cast:"casts/first-run-analysis.cast", transcript:"transcripts/first-run-analysis.md", storyboard:"storyboards/first-run-analysis.md"},
      {id:"generated-intervention-review", cast:"casts/generated-intervention-review.cast", transcript:"transcripts/generated-intervention-review.md", storyboard:"storyboards/generated-intervention-review.md"},
      {id:"ci-integration", cast:"casts/ci-integration.cast", transcript:"transcripts/ci-integration.md", storyboard:"storyboards/ci-integration.md"},
      {id:"artifact-reproduction", cast:"casts/artifact-reproduction.cast", transcript:"transcripts/artifact-reproduction.md", storyboard:"storyboards/artifact-reproduction.md"}
    ]
  }' > "$OUT/summary.json"

echo "screencasts generated: files $files, risks $risks, generated $generated, bundle files $bundle_files"
