#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/screencast-gate.json}"
OUT="${2:-results/generated/screencast-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.screencast-gate/v1" and
  (.claim | length) > 180 and
  (.required_screencasts | length) == 4 and
  (.minimum_files_scanned > 0) and
  (.minimum_ranked_risks > 0) and
  (.minimum_generated_files > 0)
' "$SPEC" > /dev/null

for phrase in "first-run analysis" "generated intervention review" "CI integration" "artifact reproduction" "make screencast-gate"; do
  grep -F "$phrase" docs/screencasts.md README.md > /dev/null
done

bash scripts/generate-screencasts.sh "$SPEC" "$OUT" > "$OUT.run.log"

min_files="$(jq '.minimum_files_scanned' "$SPEC")"
min_risks="$(jq '.minimum_ranked_risks' "$SPEC")"
min_generated="$(jq '.minimum_generated_files' "$SPEC")"
jq -e --argjson min_files "$min_files" --argjson min_risks "$min_risks" --argjson min_generated "$min_generated" '
  .version == "patchline.screencasts/v1" and
  .metrics.files_scanned >= $min_files and
  .metrics.ranked_risks >= $min_risks and
  .metrics.generated_files >= $min_generated and
  .metrics.deterministic_only == true and
  .generated_intervention.proposal_patch_lines > 0 and
  .ci.sarif_results >= 0 and
  .ci.gitlab_code_quality_findings >= 0 and
  .ci.bitbucket_annotations >= 0 and
  .reproduction.analysis_bundle_files >= 5 and
  (.reproduction.bundle_hash | length) == 64 and
  (.screencasts | length) == 4
' "$OUT/summary.json" > /dev/null

while read -r artifact; do
  test -s "$OUT/$artifact"
done < <(jq -r '.required_artifacts[] | select(. != "casts/artifact-reproduction.cast" and . != "casts/ci-integration.cast" and . != "casts/first-run-analysis.cast" and . != "casts/generated-intervention-review.cast")' "$SPEC")

while read -r slug; do
  test -s "$OUT/casts/$slug.cast"
  head -n 1 "$OUT/casts/$slug.cast" | jq -e '.version == 2 and .width >= 80 and .height >= 24' > /dev/null
  tail -n +2 "$OUT/casts/$slug.cast" | jq -e '.[1] == "o" and (.[2] | length) > 20' > /dev/null
  grep -F "$slug" "$OUT/summary.json" > /dev/null
  grep -F "$slug" "$OUT/index.md" > /dev/null
done < <(jq -r '.required_screencasts[]' "$SPEC")

grep -F "ranked data-change risks" "$OUT/casts/first-run-analysis.cast" > /dev/null
grep -F "proposal.patch" "$OUT/casts/generated-intervention-review.cast" > /dev/null
grep -F "summary.sarif" "$OUT/casts/ci-integration.cast" > /dev/null
grep -F "bundle hash" "$OUT/casts/artifact-reproduction.cast" > /dev/null
grep -F "$(jq -r '.repo' "$SPEC")" "$OUT/index.md" > /dev/null

jq -n \
  --slurpfile summary "$OUT/summary.json" \
  '{
    version:"patchline.screencast-gate-results/v1",
    screencasts:($summary[0].screencasts | length),
    files_scanned:$summary[0].metrics.files_scanned,
    ranked_risks:$summary[0].metrics.ranked_risks,
    generated_files:$summary[0].metrics.generated_files,
    analysis_bundle_files:$summary[0].reproduction.analysis_bundle_files,
    verified:true
  }' > "$OUT/gate-summary.json"

echo "screencast gate passed: screencasts $(jq '.screencasts' "$OUT/gate-summary.json"), files $(jq '.files_scanned' "$OUT/gate-summary.json"), risks $(jq '.ranked_risks' "$OUT/gate-summary.json")"
