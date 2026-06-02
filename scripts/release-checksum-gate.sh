#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/release-checksum-gate.json}"
OUT="${2:-results/generated/release-checksum-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/build-a" "$OUT/build-b" "$OUT/release" "$OUT/cache"

jq -e '
  .version == "patchline.release-checksum-gate/v1" and
  (.claim | length) > 80 and
  (.real_code.repo | length) > 0 and
  (.real_code.ref | test("^[0-9a-f]{40}$")) and
  (.real_code.subpath | length) > 0 and
  .minimum_release_artifacts >= 3
' "$SPEC" > /dev/null

grep -F "patchline release checksums" docs/release-checksums.md > /dev/null
grep -F "GOFLAGS='-trimpath -buildvcs=false'" docs/release-checksums.md > /dev/null
grep -F "make release-checksum-gate" README.md > /dev/null

go test ./cmd/patchline -run TestReleaseChecksumsSignsSortedArtifacts > "$OUT/go-test.log"

BUILD_ENV=(env CGO_ENABLED=0 GOFLAGS=-trimpath -buildvcs=false)
"${BUILD_ENV[@]}" go build -ldflags "-buildid=" -o "$OUT/build-a/patchline" ./cmd/patchline
"${BUILD_ENV[@]}" go build -ldflags "-buildid=" -o "$OUT/build-b/patchline" ./cmd/patchline
shasum -a 256 "$OUT/build-a/patchline" > "$OUT/build-a.sha256"
shasum -a 256 "$OUT/build-b/patchline" > "$OUT/build-b.sha256"
if [ "$(awk '{print $1}' "$OUT/build-a.sha256")" != "$(awk '{print $1}' "$OUT/build-b.sha256")" ]; then
  echo "reproducible build mismatch" >&2
  cat "$OUT/build-a.sha256" "$OUT/build-b.sha256" >&2
  exit 1
fi

read -r repo ref subpath < <(jq -r '[.real_code.repo, .real_code.ref, .real_code.subpath] | @tsv' "$SPEC")
go run ./cmd/patchline repo analyze \
  --github "$repo" \
  --ref "$ref" \
  --subpath "$subpath" \
  --download-dir "$OUT/cache" \
  --stages inventory,baseline,propose,compare \
  --proposal-kind all \
  --budget files=4,lines=80,tokens=12000,changes=2 \
  --no-llm \
  --out "$OUT/analyze" \
  --json > "$OUT/analyze-stdout.json"

tar -czf "$OUT/release/patchline-local-binary.tar.gz" -C "$OUT/build-a" patchline
tar -czf "$OUT/release/lobsters-analysis-bundle.tar.gz" -C "$OUT/analyze" analysis-bundle
tar -czf "$OUT/release/lobsters-proposal.tar.gz" -C "$OUT/analyze" proposal

seed="$(go run ./cmd/patchline attestation-keygen --json | jq -r '.seed_hex')"
go run ./cmd/patchline release checksums \
  --subject "patchline-release-checksum-gate" \
  --seed-hex "$seed" \
  --artifact "$OUT/release/patchline-local-binary.tar.gz" \
  --artifact "$OUT/release/lobsters-analysis-bundle.tar.gz" \
  --artifact "$OUT/release/lobsters-proposal.tar.gz" \
  --out "$OUT/checksums" \
  --json > "$OUT/release-checksums.stdout.json"

go run ./cmd/patchline verify-artifact \
  "$OUT/checksums/checksums.attestation.json" \
  --artifact "$OUT/checksums/checksums.sha256" \
  --json > "$OUT/verify.json"

jq -e --slurpfile spec "$SPEC" '
  .version == "patchline.release-checksums/v1" and
  .signature_verified == true and
  (.artifacts | length) >= $spec[0].minimum_release_artifacts and
  (.attestation.artifact_hash | length) == 64 and
  (.reproducible_build.command | contains("-trimpath")) and
  (.reproducible_build.ldflags | contains("-buildid=")) and
  (.report_hash | length) > 0
' "$OUT/checksums/release-checksums.json" > /dev/null

jq -e '.ok == true and (.artifact_hash | length) == 64' "$OUT/verify.json" > /dev/null
test "$(wc -l < "$OUT/checksums/checksums.sha256" | tr -d ' ')" -ge "$(jq '.minimum_release_artifacts' "$SPEC")"

jq -n \
  --slurpfile release "$OUT/checksums/release-checksums.json" \
  --slurpfile verify "$OUT/verify.json" \
  --slurpfile analyze "$OUT/analyze/analyze.json" \
  --rawfile build_hash "$OUT/build-a.sha256" \
  '{
    version:"patchline.release-checksum-gate-results/v1",
    release_artifacts:($release[0].artifacts | length),
    signature_verified:$release[0].signature_verified,
    attestation_verified:$verify[0].ok,
    reproducible_binary_sha256:($build_hash | split(" ")[0]),
    real_code_files_scanned:$analyze[0].summary.files_scanned,
    real_code_ranked_risks:$analyze[0].summary.ranked_risks,
    report_hash:$release[0].report_hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .signature_verified == true and .attestation_verified == true and .release_artifacts >= 3 and .real_code_ranked_risks > 0' "$OUT/summary.json" > /dev/null

echo "release checksum gate passed: $(jq '.release_artifacts' "$OUT/summary.json") signed artifacts, reproducible binary $(jq -r '.reproducible_binary_sha256' "$OUT/summary.json" | cut -c1-12), real risks $(jq '.real_code_ranked_risks' "$OUT/summary.json")"
