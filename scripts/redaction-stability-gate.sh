#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/redaction-stability-gate.json}"
OUT="${2:-results/generated/redaction-stability-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/snapshots/initial" "$OUT/snapshots/repeated" "$OUT/snapshots/resume"

jq -e '
  .version == "patchline.redaction-stability-gate/v1" and
  (.claim | length) > 80 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  (.stable_artifacts | length) >= 6
' "$SPEC" > /dev/null

grep -F "make redaction-stability-gate" docs/redaction-stability.md > /dev/null
grep -F "generated prompt" docs/redaction-stability.md > /dev/null
grep -F "comparison reports" docs/redaction-stability.md > /dev/null
grep -F "make redaction-stability-gate" README.md > /dev/null

go test ./cmd/patchline -run 'TestBundleRedactorStableAcrossInstancesAndFormats|TestRepoAnalyzeRedactionStableAcrossResume|TestBundleRedactorRemovesCanaryValues' > "$OUT/go-test.log"

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
ANALYZE=(go run ./cmd/patchline repo analyze
  --github "$repo"
  --ref "$ref"
  --subpath "$subpath"
  --download-dir "$OUT/cache"
  --stages inventory,baseline,propose,compare
  --proposal-kind all
  --budget files=4,lines=80,tokens=12000,changes=2
  --no-llm
  --redact
  --ci
  --out "$OUT/analyze"
  --json)

"${ANALYZE[@]}" > "$OUT/initial-stdout.json"

snapshot_artifacts() {
  local target="$1"
  while IFS= read -r artifact; do
    test -s "$OUT/analyze/$artifact"
    mkdir -p "$target/$(dirname "$artifact")"
    cp "$OUT/analyze/$artifact" "$target/$artifact"
  done < <(jq -r '.stable_artifacts[]' "$SPEC")
}

write_hashes() {
  local root="$1"
  local out="$2"
  while IFS= read -r artifact; do
    shasum -a 256 "$root/$artifact" | awk -v artifact="$artifact" '{print artifact "\t" $1}'
  done < <(jq -r '.stable_artifacts[]' "$SPEC") | sort > "$out"
}

snapshot_artifacts "$OUT/snapshots/initial"
write_hashes "$OUT/snapshots/initial" "$OUT/initial-hashes.tsv"

rm -rf "$OUT/analyze"
"${ANALYZE[@]}" > "$OUT/repeated-stdout.json"
snapshot_artifacts "$OUT/snapshots/repeated"
write_hashes "$OUT/snapshots/repeated" "$OUT/repeated-hashes.tsv"
cmp "$OUT/initial-hashes.tsv" "$OUT/repeated-hashes.tsv"

"${ANALYZE[@]}" --resume > "$OUT/resume-stdout.json"
snapshot_artifacts "$OUT/snapshots/resume"
write_hashes "$OUT/snapshots/resume" "$OUT/resume-hashes.tsv"
cmp "$OUT/initial-hashes.tsv" "$OUT/resume-hashes.tsv"

while IFS= read -r artifact; do
  if ! rg -F "[redacted:" "$OUT/analyze/$artifact" > /dev/null; then
    echo "expected redaction tokens in $artifact" >&2
    exit 1
  fi
done < <(jq -r '.stable_artifacts[]' "$SPEC")

jq -e '.summary.files_scanned > 0 and .summary.ranked_risks > 0 and .summary.generated_files > 0 and (.outputs.redacted_artifacts | length) > 0' "$OUT/analyze/analyze.json" > /dev/null

jq -n \
  --rawfile initial "$OUT/initial-hashes.tsv" \
  --rawfile repeated "$OUT/repeated-hashes.tsv" \
  --rawfile resume "$OUT/resume-hashes.tsv" \
  --slurpfile analyze "$OUT/analyze/analyze.json" \
  --slurpfile spec "$SPEC" \
  '{
    version:"patchline.redaction-stability-gate-results/v1",
    repo:$spec[0].real_code.repo,
    stable_artifacts:$spec[0].stable_artifacts,
    files_scanned:$analyze[0].summary.files_scanned,
    ranked_risks:$analyze[0].summary.ranked_risks,
    generated_files:$analyze[0].summary.generated_files,
    repeated_stable:($initial == $repeated),
    resume_stable:($initial == $resume),
    verified:(($initial == $repeated) and ($initial == $resume))
  }' > "$OUT/summary.json"

jq -e '.verified == true and .repeated_stable == true and .resume_stable == true and (.stable_artifacts | length) >= 6 and .ranked_risks > 0' "$OUT/summary.json" > /dev/null

echo "redaction stability gate passed: $(jq '.stable_artifacts | length' "$OUT/summary.json") artifacts stable across repeated and resume runs on $(jq -r '.repo' "$OUT/summary.json")"
