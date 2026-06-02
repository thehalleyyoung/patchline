#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/generated-provenance.json}"
OUT="${2:-results/generated/generated-provenance-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cases"

jq -e '.version == "patchline.generated-provenance/v1" and .minimum_public_slices >= 4 and (.claim | contains("fact hashes"))' "$SPEC" > /dev/null

rows=()
while IFS=$'\t' read -r id repo ref subpath; do
  case_out="$OUT/cases/$id"
  mkdir -p "$case_out"
  go run ./cmd/patchline repo analyze --github "$repo" --ref "$ref" --subpath "$subpath" --stages inventory,baseline,propose,compare --proposal-kind guards --budget files=1,lines=120,tokens=4000,changes=1 --no-llm --out "$case_out/analyze" --json > "$case_out/analyze.json"
  guard_path="$(jq -r '.generated_files[0].path' "$case_out/analyze/proposal/proposal.json")"
  content_path="$case_out/analyze/proposal/$guard_path"
  test -f "$content_path"
  rg -q -- '-- risk: risk:' "$content_path"
  rg -q -- '-- fact-hashes: sha256:' "$content_path"
  rg -q -- '-- evidence-paths: ' "$content_path"
  if rg -q 'secret|token|password|passwd|credential|api[_-]?key|private[_-]?key' "$content_path"; then
    if ! rg -q 'redacted-[0-9a-f]{8}' "$content_path"; then
      echo "secret-like provenance was not redacted in $content_path" >&2
      exit 1
    fi
  fi
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg guard_path "$guard_path" \
    --arg fact_hashes "$(rg -o 'sha256:[0-9a-f]{16}' "$content_path" | sort -u | paste -sd ',' -)" \
    --arg evidence_paths "$(sed -n 's/^-- evidence-paths: //p' "$content_path")" \
    '{id:$id, repo:$repo, subpath:$subpath, guard_path:$guard_path, fact_hashes:($fact_hashes | split(",") | map(select(length > 0))), evidence_paths:$evidence_paths, verified:true}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.slices[] | [.id, .repo, .ref, .subpath] | @tsv' examples/real-repo-slices.json)

jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile rows <(jq -s '.' "${rows[@]}") \
  '{
    version:"patchline.generated-provenance-results/v1",
    claim:$spec[0].claim,
    slices:$rows[0],
    summary:{
      public_slices:($rows[0] | length),
      artifacts_verified:($rows[0] | map(select(.verified == true)) | length),
      fact_hashes:($rows[0] | map(.fact_hashes | length) | add)
    }
  }' > "$OUT/generated-provenance.json"

jq -e --slurpfile spec "$SPEC" '
  (.slices | length) >= $spec[0].minimum_public_slices and
  .summary.artifacts_verified == (.slices | length) and
  .summary.fact_hashes >= (.slices | length) and
  all(.slices[]; .verified == true and (.fact_hashes | length) > 0 and (.evidence_paths | length) > 0)
' "$OUT/generated-provenance.json" > /dev/null

echo "generated provenance gate passed: $(jq '.summary.artifacts_verified' "$OUT/generated-provenance.json") public artifacts cite provenance"
