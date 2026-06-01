#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/generated-code-gates.json}"
OUT="${2:-results/generated/generated-code-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.generated-code-gates/v1" and
  (.features | length) > 0 and
  all(.features[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.proposal_kind | length) > 0 and
    (.bad_generated_output | length) > 0 and
    (.deterministic_checks | length) > 0 and
    (.catch_condition | length) > 20
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].features
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath proposal_kind; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"

  go run ./cmd/patchline repo fetch "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/fetch" --json > "$case_out/fetch.json"
  scan_root="$(jq -r '.source.scanned_root' "$case_out/fetch.json")"
  go run ./cmd/patchline repo inventory "$scan_root" --out "$case_out/inventory" --json > "$case_out/inventory.json"
  go run ./cmd/patchline intake "$scan_root" --out "$case_out/intake" --json > "$case_out/intake.json"
  go run ./cmd/patchline repo baseline --inventory "$case_out/inventory" --intake "$case_out/intake" --out "$case_out/baseline" --json > "$case_out/baseline.json"
  go run ./cmd/patchline repo propose --from-report "$case_out/baseline" --proposal-kind "$proposal_kind" --budget files=3,lines=120,tokens=4000,changes=3 --out "$case_out/proposal" --json > "$case_out/proposal.json"

  artifact_path="$(jq -r --arg kind "$proposal_kind" '.generated_files[] | select(.kind == $kind) | .path' "$case_out/proposal/proposal.json" | head -n 1)"
  test -n "$artifact_path"
  jq -r --arg id "$id" '.features[] | select(.id == $id) | .bad_generated_output' "$GATES" > "$case_out/proposal/$artifact_path"

  go run ./cmd/patchline repo compare --before "$case_out/baseline" --after "$case_out/proposal" --out "$case_out/compare" --json > "$case_out/compare.json"
  jq -e '.summary.patchline_checks_failed > 0 and (.intervention_loop.status | startswith("rejected"))' "$case_out/compare.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg proposal_kind "$proposal_kind" \
    --arg artifact_path "$artifact_path" \
    --slurpfile compare "$case_out/compare.json" \
    '{id: $id, repo: $repo, subpath: $subpath, proposal_kind: $proposal_kind, artifact_path: $artifact_path, patchline_checks_failed: $compare[0].summary.patchline_checks_failed, intervention_status: $compare[0].intervention_loop.status}' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.features[] | [.id, .real_repo, .subpath, .proposal_kind] | @tsv' "$GATES")

jq -s '{version:"patchline.generated-code-gate-results/v1", features: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e 'all(.features[]; .patchline_checks_failed > 0 and (.intervention_status | startswith("rejected")))' "$OUT/summary.json" > /dev/null
echo "generated-code gate passed: $(jq '.features | length' "$OUT/summary.json") generated-code entries rejected bad output with deterministic checks"
