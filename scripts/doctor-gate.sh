#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/doctor-gates.json}"
OUT="${2:-results/generated/doctor-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.doctor-gates/v1" and
  (.gates | length) >= 4 and
  all(.gates[];
    (.id | length) > 0 and
    (.real_repo | length) > 0 and
    (.subpath | length) > 0 and
    (.diagnosis_claim | length) > 40
  )
' "$GATES" > /dev/null

jq -n --slurpfile gates "$GATES" --slurpfile slices examples/real-repo-slices.json '
  ($slices[0].slices | map(.repo + ":" + .subpath)) as $known |
  $gates[0].gates
  | all(.[]; (.real_repo + ":" + .subpath) as $key | ($known | index($key)))
' | jq -e '. == true' > /dev/null

rows=()
while IFS=$'\t' read -r id repo subpath diagnosis_claim; do
  case_out="$OUT/$id"
  mkdir -p "$case_out"
  ref="$(jq -r --arg repo "$repo" --arg subpath "$subpath" '.slices[] | select(.repo == $repo and .subpath == $subpath) | .ref' examples/real-repo-slices.json)"
  test -n "$ref"
  go run ./cmd/patchline doctor --github "$repo" --ref "$ref" --subpath "$subpath" --out "$case_out/doctor" --json > "$case_out/doctor.json"
  test -s "$case_out/doctor/doctor.md"
  grep -Fq "Patchline repo doctor" "$case_out/doctor/doctor.md"
  jq -e '
    .version == "patchline.repo-doctor/v1" and
    .summary.files_scanned > 0 and
    .summary.facts >= .summary.files_scanned and
    .summary.tools_found > 0 and
    .summary.required_tools_missing == 0 and
    .summary.ready_for_analyze == true and
    .summary.network_fetch_used == true and
    (.source.resolved_commit | test("^[0-9a-f]{40}$")) and
    (.source.archive_hash | startswith("sha256:")) and
    (.cache.download_dir | length) > 0 and
    (.hash | length) > 0
  ' "$case_out/doctor.json" > /dev/null
  jq -n \
    --arg id "$id" \
    --arg repo "$repo" \
    --arg subpath "$subpath" \
    --arg diagnosis_claim "$diagnosis_claim" \
    --slurpfile doctor "$case_out/doctor.json" \
    '{
      id: $id,
      repo: $repo,
      subpath: $subpath,
      diagnosis_claim: $diagnosis_claim,
      files_scanned: $doctor[0].summary.files_scanned,
      facts: $doctor[0].summary.facts,
      tools_found: $doctor[0].summary.tools_found,
      native_checks_available: $doctor[0].summary.native_checks_available,
      ready_for_analyze: $doctor[0].summary.ready_for_analyze,
      archive_hash: $doctor[0].source.archive_hash,
      verified: true
    }' > "$case_out/row.json"
  rows+=("$case_out/row.json")
done < <(jq -r '.gates[] | [.id, .real_repo, .subpath, .diagnosis_claim] | @tsv' "$GATES")

jq -s '{version:"patchline.doctor-gate-results/v1", gates: .}' "${rows[@]}" > "$OUT/summary.json"
jq -e '(.gates | length) >= 4 and all(.gates[]; .verified == true and .ready_for_analyze == true and .files_scanned > 0 and .tools_found > 0)' "$OUT/summary.json" > /dev/null
echo "doctor gate passed: $(jq '.gates | length' "$OUT/summary.json") public repo slices diagnosed before analysis"
