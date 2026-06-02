#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/native-sandbox-profiles.json}"
OUT="${2:-results/generated/native-sandbox-profile-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '
  .version == "patchline.native-sandbox-profiles/v1" and
  .minimum_public_slices >= 4 and
  (.required_profiles | length) >= 4 and
  (.slices | length) >= .minimum_public_slices
' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath expected; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind tests --budget files=1,lines=40,tokens=1000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  jq -e --arg expected "$expected" '
    any(.native_results[]?;
      .sandbox.name == $expected and
      .sandbox.network_enabled == false and
      (.sandbox.timeout_millis > 0) and
      (.sandbox.write_scopes | index("isolated-home")) and
      (.sandbox.write_scopes | index("isolated-cache")) and
      (.sandbox.write_scopes | index("isolated-temp")) and
      (.sandbox.environment_keys | index("NO_PROXY"))
    )
  ' "$case_out/analyze/compare/compare.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg expected "$expected" \
    --slurpfile compare "$case_out/analyze/compare/compare.json" \
    '{
      id:$id,
      repo:$repo,
      expected_profile:$expected,
      native_results:$compare[0].native_results,
      matched:([$compare[0].native_results[]? | select(.sandbox.name == $expected)] | length),
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath, .expected_profile] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.native-sandbox-profile-results/v1",
    slices:$rows[0],
    required_profiles:$spec[0].required_profiles,
    summary:{
      public_slices:($rows[0] | length),
      profiles_verified:($rows[0] | map(.expected_profile) | unique | length),
      native_results:($rows[0] | map(.native_results | length) | add)
    }
  }' > "$OUT/native-sandbox-profiles.json"

jq -e --slurpfile spec "$SPEC" '
  . as $root |
  (.slices | length) >= $spec[0].minimum_public_slices and
  (.summary.profiles_verified >= ($spec[0].required_profiles | length)) and
  all($spec[0].required_profiles[]; . as $profile | any($root.slices[]; .expected_profile == $profile and .matched > 0)) and
  all(.slices[]; .verified == true and .matched > 0)
' "$OUT/native-sandbox-profiles.json" > /dev/null

echo "native sandbox profile gate passed: $(jq '.summary.profiles_verified' "$OUT/native-sandbox-profiles.json") profiles across $(jq '.summary.public_slices' "$OUT/native-sandbox-profiles.json") public repos"
