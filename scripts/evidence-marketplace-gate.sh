#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

REGISTRY="${1:-examples/evidence-marketplace/registry.json}"
OUT="${2:-results/generated/evidence-marketplace-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.evidence-marketplace/v1" and
  (.claim | length) > 140 and
  (.examples | length) >= 2 and
  all(.examples[];
    (.license_spdx | length) > 0 and
    .redaction.redaction_reviewed == true and
    .redaction.raw_data_shared == false and
    (.certificate.obligations | index("redaction-reviewed")) and
    (.certificate.obligations | index("license-cleared")) and
    (.certificate.obligations | index("artifact-hashes-verified")) and
    (.certificate.obligations | index("reproducible-without-private-data"))
  )
' "$REGISTRY" > /dev/null

for phrase in "evidence marketplace" "redacted, certificate-backed" "make evidence-marketplace-gate"; do
  grep -F "$phrase" docs/evidence-marketplace.md README.md > /dev/null
done

go test ./internal/evidencemarketplace -run 'TestPublishRegistry|TestCertificateHash|TestRenderHTML'
go test ./cmd/patchline -run TestEvidenceMarketplacePublishCommandWritesReports

go run ./cmd/patchline evidence-marketplace publish \
  --registry "$REGISTRY" \
  --out "$OUT/published" \
  --json > "$OUT/stdout.json"

test -s "$OUT/published/marketplace.json"
test -s "$OUT/published/marketplace.md"
test -s "$OUT/published/index.html"

jq -e '
  .version == "patchline.evidence-marketplace-report/v1" and
  .ok == true and
  .summary.published >= 2 and
  .summary.rejected == 0 and
  .summary.certificate_backed == .summary.published and
  .summary.redaction_reviewed == .summary.published and
  .summary.clear_licensed == .summary.published and
  .summary.artifacts_verified >= 4 and
  (.examples | all(
    (.certificate_subject_hash | startswith("sha256:")) and
    (.evidence_hash | startswith("sha256:")) and
    (.artifacts | all(.redacted == true and (.sha256 | startswith("sha256:"))))
  )) and
  (.by_hazard | length) >= 2 and
  (.by_ecosystem | length) >= 2
' "$OUT/published/marketplace.json" > /dev/null

if grep -Eiq 'password=|Authorization:|AWS_SECRET_ACCESS_KEY|source_code|BEGIN PRIVATE|token=' \
  "$OUT/published/marketplace.json" "$OUT/published/marketplace.md" "$OUT/published/index.html"; then
  echo "FAIL: marketplace output contains a high-signal private marker" >&2
  exit 1
fi

NEG="$OUT/negative-fixture"
mkdir -p "$NEG"
cp -R examples/evidence-marketplace/. "$NEG/"
jq '(.examples[0].certificate.subject_hash) = "sha256:0000000000000000000000000000000000000000000000000000000000000000"' \
  "$NEG/registry.json" > "$NEG/registry.bad.json"

set +e
go run ./cmd/patchline evidence-marketplace publish \
  --registry "$NEG/registry.bad.json" \
  --out "$OUT/rejected" \
  --json > "$OUT/rejected.stdout.json" 2> "$OUT/rejected.stderr"
status=$?
set -e
if [ "$status" -eq 0 ]; then
  echo "FAIL: corrupted certificate hash was accepted" >&2
  exit 1
fi
jq -e '.ok == false and (.rejected | length) >= 1 and (.rejected[]?.reasons[]? | contains("certificate.subject_hash mismatch"))' \
  "$OUT/rejected/marketplace.json" > /dev/null

jq -n --slurpfile r "$OUT/published/marketplace.json" '{
  version: "patchline.evidence-marketplace-gate-results/v1",
  published: $r[0].summary.published,
  artifacts_verified: $r[0].summary.artifacts_verified,
  negative_control_rejected: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "evidence-marketplace gate passed: $(jq -r .summary.published "$OUT/published/marketplace.json") redacted certificate-backed examples published; corrupted certificate rejected"
