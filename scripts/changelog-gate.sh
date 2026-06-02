#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/changelog-gate.json}"
OUT="${2:-results/generated/changelog-gate}"
CHANGELOG="${3:-CHANGELOG.md}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/proof"

jq -e '
  .version == "patchline.changelog-gate/v1" and
  .minimum_entries >= 5 and
  (.entries | length) >= .minimum_entries and
  all(.entries[];
    (.id | length) > 0 and
    (.feature | length) > 0 and
    (.surface | length) > 0 and
    (.gate | length) > 0 and
    (.proof.repo | length) > 0 and
    (.proof.ref | length) > 0 and
    (.proof.subpath | length) > 0
  ) and
  (.smoke_proof.repo | length) > 0 and
  (.smoke_proof.ref | test("^[0-9a-f]{40}$")) and
  (.smoke_proof.subpath | length) > 0
' "$SPEC" > /dev/null

grep -F "## Unreleased" "$CHANGELOG" > /dev/null
grep -F "## Changelog discipline" "$CHANGELOG" > /dev/null
grep -F "make changelog-gate" docs/changelog-discipline.md > /dev/null

rows=()
while IFS=$'\t' read -r id feature surface gate repo ref subpath; do
  grep -F "$feature" "$CHANGELOG" > /dev/null
  grep -F "$surface" "$CHANGELOG" > /dev/null
  grep -F "make $gate" "$CHANGELOG" > /dev/null
  grep -F "$repo" "$CHANGELOG" > /dev/null
  if [ "$ref" != "local-worktree" ]; then
    grep -F "$ref" "$CHANGELOG" > /dev/null
  fi
  grep -F "$subpath" "$CHANGELOG" > /dev/null
  grep -Eq "^${gate}:" Makefile
  row="$OUT/$id.json"
  jq -n \
    --arg id "$id" \
    --arg feature "$feature" \
    --arg surface "$surface" \
    --arg gate "$gate" \
    --arg repo "$repo" \
    --arg ref "$ref" \
    --arg subpath "$subpath" \
    '{id:$id,feature:$feature,surface:$surface,gate:$gate,proof:{repo:$repo,ref:$ref,subpath:$subpath},verified:true}' > "$row"
  rows+=("$row")
done < <(jq -r '.entries[] | [.id, .feature, .surface, .gate, .proof.repo, .proof.ref, .proof.subpath] | @tsv' "$SPEC")

read -r smoke_repo smoke_ref smoke_subpath < <(jq -r '[.smoke_proof.repo, .smoke_proof.ref, .smoke_proof.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo analyze \
  --github "$smoke_repo" \
  --ref "$smoke_ref" \
  --subpath "$smoke_subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --out "$OUT/proof/analyze" \
  --json > "$OUT/proof/stdout.json"

jq -e '
  .version == "patchline.repo-analyze/v1" and
  .summary.files_scanned > 0 and
  .summary.ranked_risks > 0 and
  .summary.generated_files > 0 and
  .summary.intervention_loops > 0 and
  (.hash | length) > 0
' "$OUT/proof/analyze/analyze.json" > /dev/null

jq -n \
  --slurpfile entries <(jq -s '.' "${rows[@]}") \
  --slurpfile analyze "$OUT/proof/analyze/analyze.json" \
  '{
    version:"patchline.changelog-gate-results/v1",
    entries:$entries[0],
    summary:{
      entries:($entries[0] | length),
      gates:($entries[0] | map(.gate) | unique | length),
      public_proofs:($entries[0] | map(select(.proof.ref != "local-worktree")) | length),
      smoke_files_scanned:$analyze[0].summary.files_scanned,
      smoke_ranked_risks:$analyze[0].summary.ranked_risks,
      smoke_generated_files:$analyze[0].summary.generated_files,
      smoke_intervention_loops:$analyze[0].summary.intervention_loops
    },
    verified:true
  }' > "$OUT/summary.json"

jq -e --slurpfile spec "$SPEC" '
  .verified == true and
  .summary.entries >= $spec[0].minimum_entries and
  .summary.gates == .summary.entries and
  .summary.public_proofs >= 4 and
  .summary.smoke_files_scanned > 0 and
  .summary.smoke_ranked_risks > 0 and
  .summary.smoke_generated_files > 0 and
  .summary.smoke_intervention_loops > 0 and
  all(.entries[]; .verified == true)
' "$OUT/summary.json" > /dev/null

echo "changelog gate passed: $(jq '.summary.entries' "$OUT/summary.json") entries, $(jq '.summary.public_proofs' "$OUT/summary.json") public proofs, real risks $(jq '.summary.smoke_ranked_risks' "$OUT/summary.json")"
