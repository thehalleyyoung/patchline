#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/generated-case-studies-gate.json}"
OUT="${2:-results/generated/generated-case-studies-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.generated-case-studies-gate/v1" and
  (.claim | length) > 100 and
  (.required_case_fields | length) >= 5 and
  (.real_code | length) >= 8 and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in problem evidence generated_intervention deterministic_outcome maintainer_action; do
  grep -F "$field" docs/generated-case-studies.md > /dev/null
done
grep -F "make generated-case-studies-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoCaseStudiesGenerateNarrativesFromAnalyses > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
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
    --proposal-kind all \
    --budget files=3,lines=80,tokens=12000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo case-studies \
  --analyses "$analyses" \
  --out "$OUT/case-studies" \
  --json > "$OUT/case-studies-stdout.json"

jq -e --argjson expected "$count" '
  .version == "patchline.repo-case-studies/v1" and
  .summary.cases == $expected and
  .summary.public_repos >= 8 and
  .summary.generated_artifacts > 0 and
  .summary.maintainer_actions == $expected and
  .summary.deterministic_outcomes == $expected and
  (.cases | length) == $expected and
  all(.cases[];
    (.repo | length) > 0 and
    (.ref | test("^[0-9a-f]{40}$")) and
    (.subpath | length) > 0 and
    (.problem | length) > 20 and
    (.evidence | length) > 0 and
    (.generated_intervention | contains("untrusted generated")) and
    (.deterministic_outcome | length) > 20 and
    (.maintainer_action | length) > 10 and
    (.generated_files > 0)
  )
' "$OUT/case-studies/case-studies.json" > /dev/null

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/case-studies/case-studies.md" > /dev/null
done
grep -F "generated public-repo case studies" "$OUT/case-studies/case-studies.md" > /dev/null
grep -F "maintainer action" "$OUT/case-studies/case-studies.md" > /dev/null

jq -n \
  --slurpfile cases "$OUT/case-studies/case-studies.json" \
  '{
    version:"patchline.generated-case-studies-gate-results/v1",
    cases:$cases[0].summary.cases,
    public_repos:$cases[0].summary.public_repos,
    generated_artifacts:$cases[0].summary.generated_artifacts,
    deterministic_outcomes:$cases[0].summary.deterministic_outcomes,
    maintainer_actions:$cases[0].summary.maintainer_actions,
    accepted:$cases[0].summary.accepted,
    rejected:$cases[0].summary.rejected,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .cases >= 8 and .public_repos >= 8 and .generated_artifacts > 0 and .deterministic_outcomes == .cases and .maintainer_actions == .cases' "$OUT/summary.json" > /dev/null

echo "generated case studies gate passed: cases $(jq '.cases' "$OUT/summary.json"), public repos $(jq '.public_repos' "$OUT/summary.json")"
