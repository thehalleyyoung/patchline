#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-replay.json}"
OUT="${2:-results/generated/intervention-replay-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.intervention-replay/v1" and .minimum_public_slices >= 4 and (.required_artifacts | length) >= 6 and (.claim | contains("applied diff"))' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind explain --budget files=1,lines=120,tokens=4000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  go run ./cmd/patchline repo replay --analysis "$case_out/analyze" --out "$case_out/replay" --json > "$case_out/replay.json"
  jq -e --slurpfile spec "$SPEC" '
    .version == "patchline.repo-replay/v1" and
    .patch_apply.status == "applied" and
    (.patch_apply.diff_hash | startswith("sha256:")) and
    (.hash | length) > 0
  ' "$case_out/replay.json" > /dev/null
  for artifact in $(jq -r '.required_artifacts[]' "$SPEC"); do
    jq -e --arg artifact "$artifact" 'any(.artifacts[]; .name == $artifact and (.hash | startswith("sha256:")) and .bytes > 0)' "$case_out/replay.json" > /dev/null
  done
  test -s "$case_out/replay/applied.diff"
  test -s "$case_out/replay/replay.md"
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --slurpfile replay "$case_out/replay.json" \
    '{id:$id, repo:$repo, patch_apply:$replay[0].patch_apply.status, artifacts:($replay[0].artifacts | length), hash:$replay[0].hash, verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.intervention-replay-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      verified:($rows[0] | map(select(.verified == true)) | length),
      artifacts:($rows[0] | map(.artifacts) | add)
    }
  }' > "$OUT/intervention-replay.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.verified == (.slices | length) and
  .summary.artifacts >= ((.slices | length) * ($spec[0].required_artifacts | length))
' "$OUT/intervention-replay.json" > /dev/null

echo "intervention replay gate passed: $(jq '.summary.verified' "$OUT/intervention-replay.json") public interventions replayed"
