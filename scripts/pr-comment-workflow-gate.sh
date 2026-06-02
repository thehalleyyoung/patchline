#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/pr-comment-workflow.json}"
OUT="${2:-results/generated/pr-comment-workflow-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.pr-comment-workflow/v1" and .minimum_public_repos >= 4 and (.slices | length) >= .minimum_public_repos' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline --out "$case_out/head" --json > "$case_out/head.json"
  change_key="$(jq -r '.risks[0].stable_id // .risks[0].id' "$case_out/head/baseline/baseline.json")"
  new_key="$(jq -r --arg change_key "$change_key" '[.risks[] | (.stable_id // .id) | select(. != $change_key)][0]' "$case_out/head/baseline/baseline.json")"
  jq --arg change_key "$change_key" --arg new_key "$new_key" '
    . as $head
    | .risks = ($head.risks | map(
        if ((.stable_id // .id) == $change_key) then
          .severity = "low" | .score = ((.score // 1) - 1) | .rationale = "synthetic base risk from real repo output"
        elif ((.stable_id // .id) == $new_key) then
          empty
        else
          .
        end
      ))
    | .hash = "synthetic-base-from-real-" + $head.hash
  ' "$case_out/head/baseline/baseline.json" > "$case_out/base-baseline.json"
  go run ./cmd/patchline repo pr-comment --base "$case_out/base-baseline.json" --head "$case_out/head/baseline" --max-findings 12 --out "$case_out/comment" --json > "$case_out/comment.json"
  jq -e '
    .summary.head_risks > 0 and
    .summary.new_findings > 0 and
    .summary.changed_findings > 0 and
    .summary.rendered_findings == (.findings | length) and
    all(.findings[]; .status == "new" or .status == "changed") and
    (.markdown | contains("Only new or changed data-risk findings are shown")) and
    (.markdown | contains("unchanged"))
  ' "$case_out/comment.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    --slurpfile comment "$case_out/comment.json" \
    '{
      id:$id,
      repo:$repo,
      ref:$ref,
      subpath:$subpath,
      kind:"repo-slice",
      head_risks:$comment[0].summary.head_risks,
      new_findings:$comment[0].summary.new_findings,
      changed_findings:$comment[0].summary.changed_findings,
      unchanged_omitted:$comment[0].summary.unchanged_risks,
      rendered:$comment[0].summary.rendered_findings,
      verified:true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' "$SPEC")

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.pr-comment-workflow-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_repos:($rows[0] | map(.repo) | unique | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      head_risks:($rows[0] | map(.head_risks // 0) | add),
      new_findings:($rows[0] | map(.new_findings // 0) | add),
      changed_findings:($rows[0] | map(.changed_findings // 0) | add),
      unchanged_omitted:($rows[0] | map(.unchanged_omitted // 0) | add),
      rendered:($rows[0] | map(.rendered // 0) | add)
    }
  }' > "$OUT/pr-comment-workflow.json"

jq -e --slurpfile spec "$SPEC" '
  .summary.public_repos >= $spec[0].minimum_public_repos and
  .summary.verified == (.slices | length) and
  .summary.head_risks > .summary.rendered and
  .summary.new_findings >= (.slices | length) and
  .summary.changed_findings >= (.slices | length) and
  .summary.unchanged_omitted > 0
' "$OUT/pr-comment-workflow.json" > /dev/null

echo "PR comment workflow gate passed: $(jq '.summary.public_repos' "$OUT/pr-comment-workflow.json") public repos, new=$(jq '.summary.new_findings' "$OUT/pr-comment-workflow.json"), changed=$(jq '.summary.changed_findings' "$OUT/pr-comment-workflow.json"), unchanged_omitted=$(jq '.summary.unchanged_omitted' "$OUT/pr-comment-workflow.json")"
