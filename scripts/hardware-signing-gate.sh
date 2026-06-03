#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/hardware-signing.json}"
OUT="${2:-results/generated/hardware-signing-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.hardware-signing/v1" and
  (.claim | length) > 400 and
  ([.criteria.required_artifact_kinds[]] | sort) == ["certificate","gate","release"] and
  (.signing_identities | length) == 3 and
  (.signed_artifacts | length) == 3 and
  (.drills | length) == 3 and
  .criteria.require_hardware_backing == true and
  .criteria.require_attestation == true and
  .criteria.require_threshold_approval == true and
  .criteria.require_key_rotation_drill == true and
  .criteria.require_recovery_drill == true and
  .criteria.require_revocation_drill == true and
  .criteria.require_offline_root == true
' "$SPEC" > /dev/null

for phrase in "Hardware-backed signing" "key-rotation, recovery, or revocation drills" "make hardware-signing-gate"; do
  grep -F "$phrase" docs/hardware-signing.md README.md > /dev/null
done

go test ./internal/hardwaresigning -count=1
go test ./cmd/patchline -run TestHardwareSigningCommandWritesReports -count=1

go run ./cmd/patchline hardware-signing \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/hardware-signing.json"
test -s "$OUT/safe/hardware-signing.md"

jq -e '
  .version == "patchline.hardware-signing-report/v1" and
  .ok == true and
  .summary.signing_identities == 3 and
  .summary.hardware_backed_identities == 3 and
  .summary.offline_roots == 1 and
  .summary.signed_artifacts == 3 and
  .summary.required_artifact_kinds_met == 3 and
  .summary.threshold_approved_artifacts == 3 and
  .summary.signatures == 3 and
  .summary.attestations == 3 and
  .summary.certificate_logs == 3 and
  .summary.gate_reports == 3 and
  .summary.key_rotation_drills == 1 and
  .summary.recovery_drills == 1 and
  .summary.revocation_drills == 1 and
  ([.signed_artifacts[].artifact.sha256 | select(test("^sha256:[0-9a-f]{64}$"))] | length) == 3
' "$OUT/safe/hardware-signing.json" > /dev/null

jq '
  .signing_identities = .signing_identities[:2] |
  .signing_identities[0].hardware_type = "software-file" |
  .signing_identities[0].offline_root = false |
  .signing_identities[0].attestation_path = "examples/hardware-signing/missing/release-root.attestation.json" |
  .signing_identities[0].recovery_share_paths = ["examples/hardware-signing/missing/release-root.share"] |
  .signed_artifacts = .signed_artifacts[:2] |
  .signed_artifacts[0].sha256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000" |
  .signed_artifacts[0].signature_path = "examples/hardware-signing/missing/release.sig" |
  .signed_artifacts[0].signer_ids = ["release-root", "missing-signer"] |
  .signed_artifacts[0].threshold = 3 |
  .signed_artifacts[0].certificate_log_path = "examples/hardware-signing/missing/release-log.jsonl" |
  .signed_artifacts[0].gate_report_path = "examples/hardware-signing/missing/release-gate.json" |
  .drills = .drills[:1] |
  .drills[0].evidence_paths = ["examples/hardware-signing/missing/key-rotation.md"] |
  .drills[0].result_paths = ["examples/hardware-signing/missing/key-rotation-result.json"]
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline hardware-signing \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad hardware-signing spec unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "insufficient_signing_identities")) and
  (.counterexamples | any(.kind == "missing_offline_root")) and
  (.counterexamples | any(.kind == "signer_not_hardware_backed")) and
  (.counterexamples | any(.kind == "missing_attestation")) and
  (.counterexamples | any(.kind == "missing_recovery_share")) and
  (.counterexamples | any(.kind == "missing_artifact_kind")) and
  (.counterexamples | any(.kind == "artifact_hash_mismatch")) and
  (.counterexamples | any(.kind == "missing_signature")) and
  (.counterexamples | any(.kind == "unknown_signer")) and
  (.counterexamples | any(.kind == "threshold_not_met")) and
  (.counterexamples | any(.kind == "missing_certificate_log")) and
  (.counterexamples | any(.kind == "missing_gate_report")) and
  (.counterexamples | any(.kind == "drill_missing_evidence")) and
  (.counterexamples | any(.kind == "drill_missing_result")) and
  (.counterexamples | any(.kind == "missing_drill_kind"))
' "$OUT/bad/hardware-signing.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/hardware-signing.json")"
go run ./cmd/patchline hardware-signing \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/hardware-signing.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: hardware signing report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/hardware-signing.json" --slurpfile bad "$OUT/bad/hardware-signing.json" '{
  version: "patchline.hardware-signing-gate-results/v1",
  signing_identities: $safe[0].summary.signing_identities,
  signed_artifacts: $safe[0].summary.signed_artifacts,
  threshold_approved_artifacts: $safe[0].summary.threshold_approved_artifacts,
  key_rotation_drills: $safe[0].summary.key_rotation_drills,
  recovery_drills: $safe[0].summary.recovery_drills,
  revocation_drills: $safe[0].summary.revocation_drills,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "hardware signing gate passed: release, gate, and certificate artifacts are hardware-signed, threshold-approved, drill-backed, and deterministic"
