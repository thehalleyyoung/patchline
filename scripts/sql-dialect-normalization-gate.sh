#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/sql-dialect-normalization.json}"
OUT="${2:-results/generated/sql-dialect-normalization-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.sql-dialect-normalization/v1" and .minimum_public_slices >= 4 and (.required_dialects | length) == 6' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id dialect repo ref url expect_fragment; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  curl -fsSL "$url" -o "$case_out/input.sql"
  go run ./cmd/patchline analyze-migration "$case_out/input.sql" --dialect "$dialect" --json > "$case_out/report.json"
  jq -e --arg dialect "$dialect" --arg expect "$expect_fragment" '
    .dialect == $dialect and
    (.statements | length) > 0 and
    any(.statements[]; (.normalized_sql // "") != "" and ((.fingerprint // "") == (.normalized_sql // ""))) and
    any(.statements[]; (.normalized_sql // "") | contains($expect))
  ' "$case_out/report.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg dialect "$dialect" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg expect "$expect_fragment" \
    --slurpfile report "$case_out/report.json" \
    '{
      id:$id,
      dialect:$dialect,
      repo:$repo,
      ref:$ref,
      expected_normalized_fragment:$expect,
      statements:($report[0].statements | length),
      high_risk:$report[0].summary.high_risk,
      report_hash:$report[0].summary.report_hash,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .dialect, .repo, .ref, .url, .expect_fragment] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.sql-dialect-normalization-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      dialects:($rows[0] | map(.dialect) | unique)
    }
  }' > "$OUT/sql-dialect-normalization.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.verified == (.slices | length) and
  (.summary.dialects | sort) == ($spec[0].required_dialects | sort)
' "$OUT/sql-dialect-normalization.json" > /dev/null

echo "sql dialect normalization gate passed: $(jq '.summary.verified' "$OUT/sql-dialect-normalization.json") public slices across $(jq '.summary.dialects | length' "$OUT/sql-dialect-normalization.json") dialects"
