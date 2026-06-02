#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rejected-generated-gate.json}"
OUT="${2:-results/generated/rejected-generated-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.rejected-generated-gate/v1" and
  (.claim | length) > 140 and
  (.required_fields | length) >= 8 and
  .minimum_public_repos >= 4 and
  .minimum_examples >= 4 and
  (.real_code | length) >= .minimum_public_repos and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in looks_useful_because normal_diff_appearance deterministic_rejection rejected_status failed_findings content_excerpt required_next_actions maintainer_action high_risk_generated_sql rejected_interventions; do
  grep -F "$field" docs/rejected-generated-examples.md > /dev/null
done
grep -F "make rejected-generated-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoRejectedGeneratedExamplesExplainPlausibleRejectedCode > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
llm_command='printf "%s\n" "-- Plausible generated repair for reviewer" "UPDATE comments SET updated_at = NOW();"'
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind guards \
    --budget files=3,lines=80,tokens=12000,changes=2 \
    --llm-command "$llm_command" \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo rejected-generated \
  --analyses "$analyses" \
  --out "$OUT/rejected" \
  --json > "$OUT/rejected-stdout.json"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_examples="$(jq '.minimum_examples' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_examples "$min_examples" '
  .version == "patchline.repo-rejected-generated/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.examples >= $min_examples and
  .summary.rejected_interventions >= $min_repos and
  .summary.plausible_diffs == .summary.examples and
  .summary.deterministic_rejections == .summary.examples and
  .summary.high_risk_generated_sql >= $min_examples and
  .summary.failed_generated_checks >= $min_examples and
  .summary.quarantined_generated_code >= $min_examples and
  all(.examples[];
    (.looks_useful_because | length) > 30 and
    (.normal_diff_appearance | length) > 30 and
    (.deterministic_rejection | contains("high-risk SQL")) and
    .rejected_status == "rejected-by-deterministic-checks" and
    (.review_badge | length) > 0 and
    (.failed_findings | length) > 0 and
    (.content_excerpt | length) > 0 and
    (.required_next_actions | length) > 0 and
    (.maintainer_action | length) > 30
  )
' "$OUT/rejected/rejected-generated.json" > /dev/null

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/rejected/rejected-generated.md" > /dev/null
done
grep -F "rejected generated-code examples" "$OUT/rejected/rejected-generated.md" > /dev/null
grep -F "looks useful because" "$OUT/rejected/rejected-generated.md" > /dev/null
grep -F "deterministic rejection" "$OUT/rejected/rejected-generated.md" > /dev/null

jq -n \
  --slurpfile rejected "$OUT/rejected/rejected-generated.json" \
  '{
    version:"patchline.rejected-generated-gate-results/v1",
    analyses:$rejected[0].summary.analyses,
    public_repos:$rejected[0].summary.public_repos,
    examples:$rejected[0].summary.examples,
    rejected_interventions:$rejected[0].summary.rejected_interventions,
    high_risk_generated_sql:$rejected[0].summary.high_risk_generated_sql,
    hash:$rejected[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_repos "$min_repos" --argjson min_examples "$min_examples" '.verified == true and .public_repos >= $min_repos and .examples >= $min_examples and .rejected_interventions >= $min_repos and .high_risk_generated_sql >= $min_examples' "$OUT/summary.json" > /dev/null

echo "rejected generated gate passed: examples $(jq '.examples' "$OUT/summary.json"), rejected interventions $(jq '.rejected_interventions' "$OUT/summary.json")"
