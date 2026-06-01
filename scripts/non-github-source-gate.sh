#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/non-github-source-gates.json}"
OUT="${2:-results/generated/non-github-source-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.non-github-source-gates/v1" and
  (.sources | length) >= 4 and
  ([.sources[].kind] | unique | sort) == ["archive-url","bitbucket","gitlab","sourcehut"] and
  all(.sources[];
    (.id | length) > 0 and
    (.input | length) > 0 and
    (.claim | contains("provenance")) and
    (.claim | contains("cache"))
  )
' "$GATES" > /dev/null

rows=()
while IFS=$'\t' read -r id kind input ref subpath claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  args=(repo fetch "$input" --out "$case_out/first" --download-dir "$OUT/cache" --json)
  if [ -n "$ref" ] && [ "$ref" != "-" ]; then
    args+=(--ref "$ref")
  fi
  if [ -n "$subpath" ]; then
    args+=(--subpath "$subpath")
  fi
  go run ./cmd/patchline "${args[@]}" > "$case_out/first.json"
  args=(repo fetch "$input" --out "$case_out/second" --download-dir "$OUT/cache" --json)
  if [ -n "$ref" ] && [ "$ref" != "-" ]; then
    args+=(--ref "$ref")
  fi
  if [ -n "$subpath" ]; then
    args+=(--subpath "$subpath")
  fi
  go run ./cmd/patchline "${args[@]}" > "$case_out/second.json"
  jq -e --arg kind "$kind" '
    .source.mode == $kind and
    (.source.archive_hash | startswith("sha256:")) and
    (.source.cache_key | length) > 0 and
    (.source.cache_path | length) > 0 and
    (.source.scanned_root | length) > 0
  ' "$case_out/first.json" > /dev/null
  jq -e --arg kind "$kind" '
    .source.mode == $kind and
    .source.cache_hit == true and
    (.source.archive_hash | startswith("sha256:")) and
    (.source.cache_path | length) > 0
  ' "$case_out/second.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg kind "$kind" \
    --arg input "$input" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --arg claim "$claim" \
    --slurpfile first "$case_out/first.json" \
    --slurpfile second "$case_out/second.json" \
    '{
      id: $id,
      kind: $kind,
      input: $input,
      ref: $ref,
      subpath: $subpath,
      claim: $claim,
      archive_hash: $first[0].source.archive_hash,
      cache_key: $first[0].source.cache_key,
      cache_path: $first[0].source.cache_path,
      second_fetch_cache_hit: $second[0].source.cache_hit,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.sources[] | [.id, .kind, .input, .ref, .subpath, .claim] | @tsv' "$GATES")

jq -s '{version:"patchline.non-github-source-gate-results/v1", sources: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.sources | length) >= 4 and all(.sources[]; .verified == true and .second_fetch_cache_hit == true and (.archive_hash | startswith("sha256:")))' "$OUT/summary.json" > /dev/null
echo "non-github source gate passed: $(jq '.sources | length' "$OUT/summary.json") source kinds fetched with provenance and cache reuse"
