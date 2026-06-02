#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/orm-write-effect-extraction.json}"
OUT="${2:-results/generated/orm-write-effect-extraction-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.orm-write-effect-extraction/v1" and .minimum_public_slices >= 4 and (.required_frameworks | length) == 7' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id framework repo ref url expect_operation; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  file="$case_out/$(basename "$url")"
  curl -fsSL "$url" -o "$file"
  go run ./cmd/patchline extract-sql "$file" --json > "$case_out/report.json"
  jq -e --arg framework "$framework" --arg operation "$expect_operation" '
    .summary.orm_queries > 0 and
    any(.observations[]; .kind == "orm_query" and .framework == $framework and .operation == $operation and (.effect // "") != "" and (.risk // "") != "" and ((.reasons // []) | length) > 0)
  ' "$case_out/report.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg framework "$framework" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg operation "$expect_operation" \
    --slurpfile report "$case_out/report.json" \
    '{
      id:$id,
      framework:$framework,
      repo:$repo,
      ref:$ref,
      operation:$operation,
      observations:($report[0].observations | length),
      orm_queries:$report[0].summary.orm_queries,
      hash:$report[0].hash,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .framework, .repo, .ref, .url, .expect_operation] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.orm-write-effect-extraction-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      frameworks:($rows[0] | map(.framework) | unique)
    }
  }' > "$OUT/orm-write-effect-extraction.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.verified == (.slices | length) and
  (.summary.frameworks | sort) == ($spec[0].required_frameworks | sort)
' "$OUT/orm-write-effect-extraction.json" > /dev/null

echo "orm write-effect extraction gate passed: $(jq '.summary.verified' "$OUT/orm-write-effect-extraction.json") public slices across $(jq '.summary.frameworks | length' "$OUT/orm-write-effect-extraction.json") frameworks"
