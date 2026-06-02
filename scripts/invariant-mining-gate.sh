#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/invariant-mining.json}"
OUT="${2:-results/generated/invariant-mining-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.invariant-mining/v1" and .minimum_public_repos >= 4 and (.required_sources | index("schema")) != null and (.required_sources | index("validation")) != null and (.required_sources | index("test")) != null and (.required_sources | index("fixture")) != null' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e '
    .summary.invariant_candidates > 0 and
    (.invariant_candidates | length) == .summary.invariant_candidates and
    any(.invariant_candidates[]; (.source == "schema" or .source == "validation" or .source == "test" or .source == "fixture") and (.expression | length) > 0)
  ' "$case_out/analyze/baseline/baseline.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --slurpfile baseline "$case_out/analyze/baseline/baseline.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      kind:"repo-slice",
      invariants:$baseline[0].summary.invariant_candidates,
      schema:$baseline[0].summary.invariants_from_schema,
      tests:$baseline[0].summary.invariants_from_tests,
      validations:$baseline[0].summary.invariants_from_validations,
      fixtures:$baseline[0].summary.invariants_from_fixtures,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.invariant-mining-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      invariants:($rows[0] | map(.invariants // 0) | add),
      schema:($rows[0] | map(.schema // 0) | add),
      tests:($rows[0] | map(.tests // 0) | add),
      validations:($rows[0] | map(.validations // 0) | add),
      fixtures:($rows[0] | map(.fixtures // 0) | add)
    }
  }' > "$OUT/invariant-mining.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.invariants >= (.slices | length) and
  .summary.schema > 0 and
  .summary.tests > 0 and
  .summary.validations > 0 and
  .summary.fixtures > 0
' "$OUT/invariant-mining.json" > /dev/null

echo "invariant mining gate passed: $(jq '.summary.public_repos' "$OUT/invariant-mining.json") public repos, $(jq '.summary.invariants' "$OUT/invariant-mining.json") invariants (schema=$(jq '.summary.schema' "$OUT/invariant-mining.json"), tests=$(jq '.summary.tests' "$OUT/invariant-mining.json"), validations=$(jq '.summary.validations' "$OUT/invariant-mining.json"), fixtures=$(jq '.summary.fixtures' "$OUT/invariant-mining.json"))"
