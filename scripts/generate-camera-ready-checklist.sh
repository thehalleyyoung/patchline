#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/camera-ready-checklist-gate.json}"
OUT="${2:-results/generated/camera-ready-checklist}"
rm -rf "$OUT"
mkdir -p "$OUT/evidence"

jq -e '
  .version == "patchline.camera-ready-checklist-gate/v1" and
  (.claim | length) > 120 and
  (.required_docs | length) >= 5 and
  (.required_outputs | length) >= 6 and
  (.rebuttal_spec | startswith("examples/"))
' "$SPEC" > /dev/null

rebuttal_spec="$(jq -r '.rebuttal_spec' "$SPEC")"
bash scripts/generate-rebuttal-response-workspace.sh "$rebuttal_spec" "$OUT/evidence/rebuttal-workspace" > "$OUT/evidence/rebuttal-workspace.run.log"

WORKSPACE="$OUT/evidence/rebuttal-workspace/rebuttal-workspace.json"
APPENDIX="$OUT/evidence/rebuttal-workspace/evidence/paper-appendix/appendix.json"
RELEASE="$OUT/evidence/rebuttal-workspace/evidence/release-manifest/artifact-release-manifest.json"
for file in "$WORKSPACE" "$APPENDIX" "$RELEASE"; do
  test -s "$file"
done

doc_rows=()
while read -r doc; do
  test -s "$doc"
  row="$OUT/doc-$(basename "$doc" | tr '.-' '__').json"
  jq -n \
    --arg path "$doc" \
    --arg sha "$(shasum -a 256 "$doc" | awk '{print $1}')" \
    --argjson bytes "$(wc -c < "$doc" | tr -d ' ')" \
    '{path:$path, sha256:$sha, bytes:$bytes}' > "$row"
  doc_rows+=("$row")
done < <(jq -r '.required_docs[]' "$SPEC")

appendix_claims="$(jq '.claims | length' "$APPENDIX")"
appendix_summary_claims="$(jq '.summary.claims' "$APPENDIX")"
appendix_figures="$(jq '.figures | length' "$APPENDIX")"
appendix_summary_figures="$(jq '.summary.figures' "$APPENDIX")"
appendix_tables="$(jq '.tables | length' "$APPENDIX")"
appendix_summary_tables="$(jq '.summary.tables' "$APPENDIX")"
workspace_concerns="$(jq '.responses | length' "$WORKSPACE")"
workspace_summary_concerns="$(jq '.summary.concerns' "$WORKSPACE")"
release_archives="$(jq '.archives | length' "$RELEASE")"
release_summary_archives="$(jq '.summary.archives' "$RELEASE")"

jq -n \
  --slurpfile docs <(jq -s '.' "${doc_rows[@]}") \
  --slurpfile workspace "$WORKSPACE" \
  --slurpfile appendix "$APPENDIX" \
  --slurpfile release "$RELEASE" \
  --argjson appendix_claims "$appendix_claims" \
  --argjson appendix_summary_claims "$appendix_summary_claims" \
  --argjson appendix_figures "$appendix_figures" \
  --argjson appendix_summary_figures "$appendix_summary_figures" \
  --argjson appendix_tables "$appendix_tables" \
  --argjson appendix_summary_tables "$appendix_summary_tables" \
  --argjson workspace_concerns "$workspace_concerns" \
  --argjson workspace_summary_concerns "$workspace_summary_concerns" \
  --argjson release_archives "$release_archives" \
  --argjson release_summary_archives "$release_summary_archives" \
  '{
    version:"patchline.camera-ready-checklist/v1",
    checks:[
      {id:"claims-count", category:"claims", passed:($appendix_claims == $appendix_summary_claims), expected:$appendix_summary_claims, actual:$appendix_claims},
      {id:"figures-count", category:"figures", passed:($appendix_figures == $appendix_summary_figures), expected:$appendix_summary_figures, actual:$appendix_figures},
      {id:"tables-count", category:"tables", passed:($appendix_tables == $appendix_summary_tables), expected:$appendix_summary_tables, actual:$appendix_tables},
      {id:"rebuttal-concerns-count", category:"rebuttal", passed:($workspace_concerns == $workspace_summary_concerns), expected:$workspace_summary_concerns, actual:$workspace_concerns},
      {id:"release-archives-count", category:"release", passed:($release_archives == $release_summary_archives), expected:$release_summary_archives, actual:$release_archives},
      {id:"public-repo-agreement", category:"evidence", passed:($appendix[0].summary.public_repos == $release[0].summary.public_repos and $appendix[0].summary.public_repos == $workspace[0].summary.public_repos), expected:$appendix[0].summary.public_repos, actual:$release[0].summary.public_repos},
      {id:"ranked-risk-agreement", category:"evidence", passed:($appendix[0].summary.ranked_risks == $workspace[0].summary.ranked_risks), expected:$appendix[0].summary.ranked_risks, actual:$workspace[0].summary.ranked_risks},
      {id:"release-content-hash", category:"release", passed:(($release[0].release.content_hash // "") | test("^sha256:[0-9a-f]{64}$")), expected:"sha256 hash", actual:($release[0].release.content_hash // "")},
      {id:"docs-present", category:"docs", passed:all($docs[0][]; .bytes > 0 and (.sha256 | test("^[0-9a-f]{64}$"))), expected:($docs[0] | length), actual:($docs[0] | length)},
      {id:"workspace-answerable", category:"rebuttal", passed:($workspace[0].summary.answerable == $workspace[0].summary.concerns), expected:$workspace[0].summary.concerns, actual:$workspace[0].summary.answerable}
    ],
    docs:$docs[0],
    evidence:{
      workspace:"evidence/rebuttal-workspace/rebuttal-workspace.json",
      appendix:"evidence/rebuttal-workspace/evidence/paper-appendix/appendix.json",
      release_manifest:"evidence/rebuttal-workspace/evidence/release-manifest/artifact-release-manifest.json"
    },
    summary:{
      checks:10,
      passed:0,
      failed:0,
      public_repos:$appendix[0].summary.public_repos,
      claims:$appendix[0].summary.claims,
      figures:$appendix[0].summary.figures,
      tables:$appendix[0].summary.tables,
      ranked_risks:$appendix[0].summary.ranked_risks,
      release_blocked:false,
      verified:false
    }
  }' > "$OUT/checklist-raw.json"

jq '
  .summary.passed = ([.checks[] | select(.passed == true)] | length) |
  .summary.failed = ([.checks[] | select(.passed == false)] | length) |
  .summary.release_blocked = (([.checks[] | select(.passed == false)] | length) > 0) |
  .summary.verified = (([.checks[] | select(.passed == false)] | length) == 0)
' "$OUT/checklist-raw.json" > "$OUT/camera-ready-checklist.json"
rm "$OUT/checklist-raw.json"

jq -n '{
  version:"patchline.camera-ready-drift-policy/v1",
  block_release_when:[
    "claim counts drift from appendix summary",
    "figure counts drift from generated figure summary",
    "table counts drift from generated table summary",
    "documentation files are missing or hashless",
    "release manifest archives or content hashes drift",
    "rebuttal responses are no longer answerable with linked evidence and limitations"
  ]
}' > "$OUT/drift-policy.json"

{
  echo "# Camera-ready checklist"
  echo
  echo "This checklist blocks release if claims, figures, tables, docs, rebuttal responses, or release metadata drift from generated evidence."
  echo
  jq -r '.summary | "- checks: `" + (.checks|tostring) + "`\n- passed: `" + (.passed|tostring) + "`\n- failed: `" + (.failed|tostring) + "`\n- release blocked: `" + (.release_blocked|tostring) + "`\n- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`"' "$OUT/camera-ready-checklist.json"
  echo
  echo "## Checks"
  jq -r '.checks[] | "- `" + .id + "` (" + .category + "): passed=`" + (.passed|tostring) + "`, expected=`" + (.expected|tostring) + "`, actual=`" + (.actual|tostring) + "`"' "$OUT/camera-ready-checklist.json"
} > "$OUT/camera-ready-checklist.md"

cp "$OUT/camera-ready-checklist.md" "$OUT/README.md"
echo "camera-ready checklist generated: checks $(jq '.summary.checks' "$OUT/camera-ready-checklist.json"), failed $(jq '.summary.failed' "$OUT/camera-ready-checklist.json"), risks $(jq '.summary.ranked_risks' "$OUT/camera-ready-checklist.json")"
