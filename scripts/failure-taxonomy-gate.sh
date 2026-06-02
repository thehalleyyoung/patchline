#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/failure-taxonomy-gate.json}"
OUT="${2:-results/generated/failure-taxonomy-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.failure-taxonomy-gate/v1" and
  (.claim | length) > 120 and
  (.required_fields | length) >= 7 and
  .minimum_failure_modes >= 5 and
  .minimum_public_repos >= 8 and
  (.real_code | length) >= 8 and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in failure_modes definition repair_risk maintainer_decision examples evidence_kinds summary; do
  grep -F "$field" docs/failure-mode-taxonomy.md > /dev/null
done
grep -F "make failure-taxonomy-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoTaxonomyClassifiesFailureModesFromAnalyses > "$OUT/go-test.log"

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

go run ./cmd/patchline repo taxonomy \
  --analyses "$analyses" \
  --out "$OUT/taxonomy" \
  --json > "$OUT/taxonomy-stdout.json"

min_modes="$(jq '.minimum_failure_modes' "$SPEC")"
min_repos="$(jq '.minimum_public_repos' "$SPEC")"
jq -e --argjson min_modes "$min_modes" --argjson min_repos "$min_repos" '
  .version == "patchline.repo-failure-taxonomy/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.failure_modes >= $min_modes and
  .summary.occurrences >= .summary.failure_modes and
  .summary.generated_intervention_links > 0 and
  (.failure_modes | length) == .summary.failure_modes and
  all(.failure_modes[];
    (.id | length) > 0 and
    (.definition | length) > 30 and
    (.repair_risk | length) > 20 and
    (.maintainer_decision | length) > 20 and
    (.occurrences > 0) and
    (.public_repos > 0) and
    (.examples | length) > 0 and
    (.evidence_kinds | length) > 0
  )
' "$OUT/taxonomy/failure-taxonomy.json" > /dev/null

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/taxonomy/failure-taxonomy.md" > /dev/null
done
grep -F "public-corpus failure-mode taxonomy" "$OUT/taxonomy/failure-taxonomy.md" > /dev/null
grep -F "maintainer decision" "$OUT/taxonomy/failure-taxonomy.md" > /dev/null

jq -n \
  --slurpfile taxonomy "$OUT/taxonomy/failure-taxonomy.json" \
  '{
    version:"patchline.failure-taxonomy-gate-results/v1",
    analyses:$taxonomy[0].summary.analyses,
    public_repos:$taxonomy[0].summary.public_repos,
    failure_modes:$taxonomy[0].summary.failure_modes,
    occurrences:$taxonomy[0].summary.occurrences,
    generated_intervention_links:$taxonomy[0].summary.generated_intervention_links,
    hash:$taxonomy[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_modes "$min_modes" --argjson min_repos "$min_repos" '.verified == true and .public_repos >= $min_repos and .failure_modes >= $min_modes and .generated_intervention_links > 0' "$OUT/summary.json" > /dev/null

echo "failure taxonomy gate passed: modes $(jq '.failure_modes' "$OUT/summary.json"), public repos $(jq '.public_repos' "$OUT/summary.json")"
