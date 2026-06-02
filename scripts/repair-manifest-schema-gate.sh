#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/repair-manifest-schema.json}"
OUT="${2:-results/generated/repair-manifest-schema-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.repair-manifest-schema/v1" and .minimum_public_slices >= 4 and (.required_fields | length) >= 10' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind repair --budget files=1,lines=160,tokens=4000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  manifest_path="$(jq -r '.generated_files[0].path' "$case_out/analyze/proposal/proposal.json")"
  content_path="$case_out/analyze/proposal/$manifest_path"
  test -f "$content_path"
  jq -e '
    .version == "patchline.generated-repair/v1" and
    .trust == "untrusted-generated-proposal" and
    (.risk_id | length) > 0 and
    (.source | length) > 0 and
    (.scope.table | length) > 0 and
    (.scope.where | length) > 0 and
    (.preconditions | length) > 0 and
    (.postconditions | length) > 0 and
    .rollback.required == true and
    (.rollback.strategy | length) > 0 and
    (.rollback.steps | length) > 0 and
    (.validation_commands | length) > 0 and
    all(.validation_commands[]; (.command | length) > 0 and (.reason | length) > 0) and
    .owner_review.required == true and
    (.owner_review.status == "pending" or .owner_review.status == "approved" or .owner_review.status == "rejected") and
    (.owner_review.owner | length) > 0
  ' "$content_path" > /dev/null
  jq -e --arg path "$manifest_path" '.summary.patchline_checks_failed == 0 and any(.generated_checks[]; .path == $path and .status == "pass")' "$case_out/analyze/compare/compare.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg manifest_path "$manifest_path" \
    --slurpfile manifest "$content_path" \
    '{id:$id, repo:$repo, subpath:$subpath, manifest_path:$manifest_path, risk_id:$manifest[0].risk_id, owner_review:$manifest[0].owner_review.status, validation_commands:($manifest[0].validation_commands | length), rollback_steps:($manifest[0].rollback.steps | length), verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.repair-manifest-schema-results/v1",
    required_fields:$spec[0].required_fields,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      manifests_verified:($rows[0] | map(select(.verified == true)) | length),
      validation_commands:($rows[0] | map(.validation_commands) | add),
      rollback_steps:($rows[0] | map(.rollback_steps) | add)
    }
  }' > "$OUT/repair-manifest-schema.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.manifests_verified == (.slices | length) and
  .summary.validation_commands >= (.slices | length) and
  .summary.rollback_steps >= (.slices | length)
' "$OUT/repair-manifest-schema.json" > /dev/null

echo "repair manifest schema gate passed: $(jq '.summary.manifests_verified' "$OUT/repair-manifest-schema.json") manifests verified"
