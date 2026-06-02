#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/archive-security-gate.json}"
OUT="${2:-results/generated/archive-security-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.archive-security-gate/v1" and
  (.claim | length) > 100 and
  (.focused_tests | length) >= 6 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  (.required_controls | length) >= 5
' "$SPEC" > /dev/null

while IFS= read -r control; do
  grep -F "$control" docs/archive-security.md > /dev/null
done < <(jq -r '.required_controls[]' "$SPEC")
grep -F "make archive-security-gate" README.md > /dev/null

go test ./internal/project -run 'TestExtract(ZipRejectsUnsafePaths|TarGzRejectsUnsafePaths|ArchivesIgnoreSymlinkEscapes|ArchivesRejectMalformedInputs|ArchivesRejectBombs|ArchivesAcceptValidRepoFiles)$' > "$OUT/go-test.log"

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo fetch "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --out "$OUT/fetch-first" \
  --json > "$OUT/fetch-first.json"

go run ./cmd/patchline repo fetch "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --out "$OUT/fetch-second" \
  --json > "$OUT/fetch-second.json"

jq -e --arg ref "$ref" --arg subpath "$subpath" '
  .version == "patchline.project/v1" and
  .mode == "github" and
  .resolved_commit == $ref and
  .subpath == $subpath and
  (.archive_hash | length) > 20 and
  (.cache_path | length) > 0 and
  (.scanned_root | length) > 0
' "$OUT/fetch-first/source.json" > /dev/null

jq -e '
  .version == "patchline.project/v1" and
  .mode == "github" and
  .cache_hit == true and
  (.archive_hash | length) > 20 and
  (.cache_path | length) > 0
' "$OUT/fetch-second/source.json" > /dev/null

scanned_root="$(jq -r '.scanned_root' "$OUT/fetch-second/source.json")"
test -d "$scanned_root"
test -n "$(find "$scanned_root" -type f | head -n 1)"

jq -n \
  --slurpfile first "$OUT/fetch-first/source.json" \
  --slurpfile second "$OUT/fetch-second/source.json" \
  '{
    version:"patchline.archive-security-gate-results/v1",
    repo:$first[0].input,
    ref:$first[0].resolved_commit,
    subpath:$first[0].subpath,
    archive_hash:$first[0].archive_hash,
    first_cache_hit:$first[0].cache_hit,
    second_cache_hit:$second[0].cache_hit,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .second_cache_hit == true and (.archive_hash | length) > 20' "$OUT/summary.json" > /dev/null

echo "archive security gate passed: repo $(jq -r '.repo' "$OUT/summary.json"), cache reuse $(jq -r '.second_cache_hit' "$OUT/summary.json")"
