#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/benchmarks/stratified-public-catalog.json}"
OUT="${2:-results/generated/stratified-benchmark-manifests}"
rm -rf "$OUT"
mkdir -p "$OUT/ecosystem" "$OUT/migration_framework"

catalog="$(jq -r '.source_catalog' "$SPEC")"
jq -e '.version == "patchline.stratified-benchmark-spec/v1" and (.dimensions | index("ecosystem")) and (.dimensions | index("migration_framework"))' "$SPEC" > /dev/null
jq -e '.version == "patchline.real-repo-catalog/v1" and (.slices | length) >= 25' "$catalog" > /dev/null

make_cases='
  def command:
    "go run ./cmd/patchline repo analyze --github " + .repo +
    " --ref " + .ref +
    " --subpath " + .subpath +
    " --stages inventory,baseline,propose,compare --no-llm --out results/generated/stratified/" + .id;
  map({
    id,
    repo,
    ref,
    subpath,
    ecosystem,
    migration_framework,
    source_host,
    monorepo,
    command: command
  })
'

for dimension in ecosystem migration_framework; do
  values="$(jq -r --arg dimension "$dimension" '[.slices[].[$dimension]] | unique[]' "$catalog")"
  while IFS= read -r value; do
    [ -n "$value" ] || continue
    slug="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g; s/--*/-/g; s/^-//; s/-$//')"
    jq --arg dimension "$dimension" --arg value "$value" --arg make_cases "$make_cases" '
      {
        version: "patchline.stratified-benchmark-manifest/v1",
        stratum: {dimension: $dimension, value: $value},
        cases: ((.slices | map(select(.[$dimension] == $value))) | '"$make_cases"')
      }
    ' "$catalog" > "$OUT/$dimension/$slug.json"
  done <<< "$values"
done

jq -n \
  --slurpfile catalog "$catalog" \
  --arg ecosystem_dir "$OUT/ecosystem" \
  --arg framework_dir "$OUT/migration_framework" \
  '{
    version:"patchline.stratified-benchmark-summary/v1",
    source_slices: ($catalog[0].slices | length),
    ecosystem_manifests: ($catalog[0].slices | map(.ecosystem) | unique | length),
    framework_manifests: ($catalog[0].slices | map(.migration_framework) | unique | length),
    ecosystem_dir: $ecosystem_dir,
    framework_dir: $framework_dir
  }' > "$OUT/summary.json"

for file in "$OUT"/ecosystem/*.json "$OUT"/migration_framework/*.json; do
  jq -e '
    .version == "patchline.stratified-benchmark-manifest/v1" and
    (.stratum.dimension == "ecosystem" or .stratum.dimension == "migration_framework") and
    (.stratum.value | length) > 0 and
    (.cases | length) > 0 and
    all(.cases[]; (.id | length) > 0 and (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.command | contains("repo analyze")))
  ' "$file" > /dev/null
done

jq -e '.source_slices >= 25 and .ecosystem_manifests >= 7 and .framework_manifests >= 12' "$OUT/summary.json" > /dev/null
echo "stratified benchmark gate passed: $(jq '.ecosystem_manifests' "$OUT/summary.json") ecosystem and $(jq '.framework_manifests' "$OUT/summary.json") framework manifests"
