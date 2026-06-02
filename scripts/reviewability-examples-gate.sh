#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewability-examples-gate.json}"
OUT="${2:-results/generated/reviewability-examples-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.reviewability-examples-gate/v1" and
  (.claim | length) > 160 and
  (.required_fields | length) >= 7 and
  .minimum_public_repos >= 4 and
  .minimum_examples >= 8 and
  (.real_code | length) >= .minimum_public_repos and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in reviewability_gain non_repair_claim deterministic_outcome proof_holes content_excerpt maintainer_action required_reanalysis test_examples guard_examples no_full_repair_claims; do
  grep -F "$field" docs/reviewability-examples.md > /dev/null
done
grep -F "make reviewability-examples-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoReviewabilityExamplesDoNotClaimFullRepair > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  proposal_kind="guards"
  if (( i % 2 == 0 )); then
    proposal_kind="tests"
  fi
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind "$proposal_kind" \
    --budget files=12,lines=120,tokens=16000,changes=3 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo reviewability-examples \
  --analyses "$analyses" \
  --out "$OUT/examples" \
  --json > "$OUT/examples-stdout.json"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
min_examples="$(jq '.minimum_examples' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson min_examples "$min_examples" '
  .version == "patchline.repo-reviewability-examples/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.examples >= $min_examples and
  .summary.test_examples > 0 and
  .summary.guard_examples > 0 and
  .summary.accepted_for_review > 0 and
  .summary.proof_holes_listed > 0 and
  .summary.no_full_repair_claims == .summary.examples and
  .summary.deterministic_checks_passed >= .summary.examples and
  all(.examples[];
    (.reviewability_gain | length) > 30 and
    (.non_repair_claim | contains("does not claim to repair")) and
    (.deterministic_outcome | length) > 20 and
    (.proof_holes | length) > 0 and
    (.content_excerpt | length) > 0 and
    (.maintainer_action | length) > 30 and
    (.required_reanalysis | length) > 0
  )
' "$OUT/examples/reviewability-examples.json" > /dev/null

for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/examples/reviewability-examples.md" > /dev/null
done
grep -F "generated reviewability examples" "$OUT/examples/reviewability-examples.md" > /dev/null
grep -F "non-repair claim" "$OUT/examples/reviewability-examples.md" > /dev/null
grep -F "proof holes preserved" "$OUT/examples/reviewability-examples.md" > /dev/null

jq -n \
  --slurpfile examples "$OUT/examples/reviewability-examples.json" \
  '{
    version:"patchline.reviewability-examples-gate-results/v1",
    analyses:$examples[0].summary.analyses,
    public_repos:$examples[0].summary.public_repos,
    examples:$examples[0].summary.examples,
    test_examples:$examples[0].summary.test_examples,
    guard_examples:$examples[0].summary.guard_examples,
    no_full_repair_claims:$examples[0].summary.no_full_repair_claims,
    hash:$examples[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_repos "$min_repos" --argjson min_examples "$min_examples" '.verified == true and .public_repos >= $min_repos and .examples >= $min_examples and .test_examples > 0 and .guard_examples > 0 and .no_full_repair_claims == .examples' "$OUT/summary.json" > /dev/null

echo "reviewability examples gate passed: examples $(jq '.examples' "$OUT/summary.json"), tests $(jq '.test_examples' "$OUT/summary.json"), guards $(jq '.guard_examples' "$OUT/summary.json")"
