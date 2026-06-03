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
    (.certificate.obligations | index("reproducible-without-private-data")) and
    (.gate_reputation | keys == ["first_verified_at","independent_confirmations","last_verified_at","reproducible_runs"]) and
    .gate_reputation.reproducible_runs >= 0 and
    (.gate_reputation.independent_confirmations | length) >= 2
  )
' "$REGISTRY" > /dev/null

for phrase in "evidence marketplace" "redacted, certificate-backed" "archive mirror" "reproducibility, longevity, and independent confirmation" "make evidence-marketplace-gate"; do
  grep -F "$phrase" docs/evidence-marketplace.md README.md > /dev/null
done

go test ./internal/evidencemarketplace -run 'TestPublishRegistry|TestCertificateHash|TestRenderHTML|TestWriteReport'
go test ./internal/artifact -run TestImportMarketplaceBenchmark
go test ./cmd/patchline -run 'TestEvidenceMarketplacePublishCommandWritesReports|TestArtifactBenchmarkImportMarketplaceCommandWritesRunnableBenchmark'

go run ./cmd/patchline evidence-marketplace publish \
  --registry "$REGISTRY" \
  --out "$OUT/published" \
  --json > "$OUT/stdout.json"

test -s "$OUT/published/marketplace.json"
test -s "$OUT/published/marketplace.md"
test -s "$OUT/published/index.html"
test -s "$OUT/published/archive-mirror.json"

jq -e '
  .version == "patchline.evidence-marketplace-report/v1" and
  .ok == true and
  .summary.published >= 2 and
  .summary.rejected == 0 and
  .summary.certificate_backed == .summary.published and
  .summary.redaction_reviewed == .summary.published and
  .summary.clear_licensed == .summary.published and
  .summary.public_release_eligible == .summary.published and
  .summary.prevalence_examples == .summary.published and
  .summary.duplicate_inflation == 0 and
  .summary.exact_duplicate_groups == 0 and
  .summary.near_duplicate_groups == 0 and
  .summary.artifacts_verified >= 4 and
  .summary.mirrored_artifacts == .summary.artifacts_verified and
  .summary.mirror_bytes > 0 and
  .summary.gate_reputation_submitted == .summary.published and
  .summary.gate_reputation_reviewable == .summary.published and
  .summary.gate_reputation_established >= 1 and
  .archive_mirror.version == "patchline.evidence-marketplace-mirror/v1" and
  .archive_mirror.summary.artifacts == .summary.artifacts_verified and
  .archive_mirror.summary.unique_files == .summary.artifacts_verified and
  .archive_mirror.summary.active == .summary.artifacts_verified and
  .archive_mirror.summary.withdrawn == 0 and
  (.archive_mirror.entries | all(
    (.mirror_path | startswith("archive/sha256/")) and
    (.checksum | startswith("sha256:")) and
    (.license_spdx | length) > 0 and
    .redacted == true and
    (.withdrawal.status == "active") and
    (.withdrawal.requested == false) and
    (.withdrawal.withdrawal_id | startswith("sha256:")) and
    (.withdrawal.policy_url | length) > 0 and
    (.withdrawal.contact | length) > 0 and
    (.withdrawal.review_required == true) and
    (.withdrawal.tombstone_required == true) and
    (.withdrawal.preserve_checksum_after_withdrawal == true)
  )) and
  (.examples | all(
    (.certificate_subject_hash | startswith("sha256:")) and
    (.evidence_hash | startswith("sha256:")) and
    (.release_admission.public_release_eligible == true) and
    (.release_admission.license_accepted == true) and
    (.release_admission.consent_names_submitter == true) and
    (.release_admission.consent_grants_publication == true) and
    (.release_admission.consent_names_license == true) and
    (.gate_reputation.score >= 50) and
    ((.gate_reputation.tier == "reviewable") or (.gate_reputation.tier == "established")) and
    (.gate_reputation.reproducibility_points >= 28) and
    (.gate_reputation.longevity_points >= 15) and
    (.gate_reputation.confirmation_points >= 20) and
    (.duplicate_analysis.prevalence_representative == true) and
    (.duplicate_analysis.prevalence_weight == 1) and
    (.duplicate_analysis.exact_fingerprint | startswith("sha256:")) and
    (.duplicate_analysis.near_fingerprint | startswith("sha256:")) and
    (.artifacts | all(.redacted == true and (.sha256 | startswith("sha256:"))))
  )) and
  (.by_hazard | length) >= 2 and
  (.by_hazard_prevalence | length) >= 2 and
  (.by_ecosystem | length) >= 2 and
  (.by_reputation_tier | length) >= 2
' "$OUT/published/marketplace.json" > /dev/null

jq -e '
  .version == "patchline.evidence-marketplace-mirror/v1" and
  .summary.artifacts >= 4 and
  .summary.active == .summary.artifacts and
  .summary.withdrawn == 0 and
  (.entries | all(
    (.checksum | startswith("sha256:")) and
    (.license_spdx | length) > 0 and
    (.certificate_subject_hash | startswith("sha256:")) and
    (.evidence_hash | startswith("sha256:")) and
    (.withdrawal.withdrawal_id | startswith("sha256:"))
  ))
' "$OUT/published/archive-mirror.json" > /dev/null

while IFS=$'\t' read -r checksum mirror_path; do
  test -f "$OUT/published/$mirror_path"
  actual="sha256:$(shasum -a 256 "$OUT/published/$mirror_path" | cut -d' ' -f1)"
  if [ "$actual" != "$checksum" ]; then
    echo "FAIL: mirrored artifact $mirror_path checksum $actual did not match manifest $checksum" >&2
    exit 1
  fi
done < <(jq -r '.entries[] | [.checksum, .mirror_path] | @tsv' "$OUT/published/archive-mirror.json")

if grep -Eiq 'password=|Authorization:|AWS_SECRET_ACCESS_KEY|source_code|BEGIN PRIVATE|token=' \
  "$OUT/published/marketplace.json" "$OUT/published/marketplace.md" "$OUT/published/index.html" "$OUT/published/archive-mirror.json" "$OUT/published/archive/sha256/"*; then
  echo "FAIL: marketplace output contains a high-signal private marker" >&2
  exit 1
fi

go run ./cmd/patchline artifact-benchmark import-marketplace \
  --registry "$REGISTRY" \
  --out "$OUT/imported-benchmark" \
  --json > "$OUT/imported-benchmark.stdout.json"

test -s "$OUT/imported-benchmark/marketplace-import.json"
test -s "$OUT/imported-benchmark/manifests/marketplace-import.json"

jq -e '
  .version == "patchline.marketplace-benchmark-import/v1" and
  .ok == true and
  .summary.imported >= 2 and
  .summary.rejected == 0 and
  .summary.duplicate_imports_skipped == 0 and
  (.cases | all(
    .submitter_labels_trusted == false and
    .label_source == "artifact-evidence-cue" and
    (.certificate_subject_hash | startswith("sha256:")) and
    (.marketplace_artifact_hash | startswith("sha256:")) and
    (.fixture_sha256 | startswith("sha256:")) and
    (.ground_truth_sha256 | startswith("sha256:"))
  ))
' "$OUT/imported-benchmark/marketplace-import.json" > /dev/null

go run ./cmd/patchline artifact-benchmark validate \
  "$OUT/imported-benchmark/manifests/marketplace-import.json" \
  --json > "$OUT/imported-benchmark.validate.json"
go run ./cmd/patchline artifact-benchmark run \
  "$OUT/imported-benchmark/manifests/marketplace-import.json" \
  --out "$OUT/imported-benchmark/run.json" \
  --json > "$OUT/imported-benchmark.run.stdout.json"

jq -e '.ok == true and .metrics.total >= 2 and .metrics.failed == 0 and (.cases | all(.actual_result == "flag"))' \
  "$OUT/imported-benchmark/run.json" > /dev/null

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
jq -e '.version == "patchline.evidence-marketplace-mirror/v1" and .summary.artifacts == 2 and (.entries | length) == 2 and (.entries | all(.example_id == "redacted-django-nullability-backfill"))' \
  "$OUT/rejected/archive-mirror.json" > /dev/null

jq -n \
  --slurpfile r "$OUT/published/marketplace.json" \
  --slurpfile m "$OUT/published/archive-mirror.json" \
  --slurpfile i "$OUT/imported-benchmark/marketplace-import.json" '{
  version: "patchline.evidence-marketplace-gate-results/v1",
  published: $r[0].summary.published,
  prevalence_examples: $r[0].summary.prevalence_examples,
  duplicate_inflation: $r[0].summary.duplicate_inflation,
  artifacts_verified: $r[0].summary.artifacts_verified,
  mirrored_artifacts: $m[0].summary.artifacts,
  public_release_eligible: $r[0].summary.public_release_eligible,
  gate_reputation_reviewable: $r[0].summary.gate_reputation_reviewable,
  gate_reputation_established: $r[0].summary.gate_reputation_established,
  imported_benchmark_cases: $i[0].summary.imported,
  negative_control_rejected: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "evidence-marketplace gate passed: $(jq -r .summary.published "$OUT/published/marketplace.json") redacted certificate-backed examples published; $(jq -r .summary.prevalence_examples "$OUT/published/marketplace.json") prevalence examples after duplicate collapse; $(jq -r .summary.artifacts "$OUT/published/archive-mirror.json") artifacts mirrored with checksum/license/withdrawal metadata; $(jq -r .summary.gate_reputation_reviewable "$OUT/published/marketplace.json") reviewable gate reputations scored; $(jq -r .summary.imported "$OUT/imported-benchmark/marketplace-import.json") imported into runnable benchmarks; corrupted certificate rejected"
