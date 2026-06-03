#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/confidential-computing.json}"
OUT="${2:-results/generated/confidential-computing-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.confidential-computing/v1" and
  (.claim | length) > 400 and
  ([.criteria.required_tee_kinds[]] | sort) == ["sev-snp","sgx"] and
  (.enclaves | length) == 2 and
  (.key_release_policies | length) == 2 and
  (.workloads | length) == 2 and
  .criteria.require_attestation == true and
  .criteria.require_measurement_allowlist == true and
  .criteria.require_key_release_policy == true and
  .criteria.require_encrypted_inputs == true and
  .criteria.require_private_outputs == true and
  .criteria.require_no_plaintext_export == true and
  .criteria.require_no_network_egress == true and
  .criteria.require_verifier_evidence == true and
  .criteria.require_replay_evidence == true
' "$SPEC" > /dev/null

for phrase in "Confidential-computing evaluation" "private corpus analysis" "verifiable enclave attestation" "make confidential-computing-gate"; do
  grep -F "$phrase" docs/confidential-computing.md README.md > /dev/null
done

go test ./internal/confidentialcomputing -count=1
go test ./cmd/patchline -run TestConfidentialComputingCommandWritesReports -count=1

go run ./cmd/patchline confidential-computing \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/confidential-computing.json"
test -s "$OUT/safe/confidential-computing.md"

jq -e '
  .version == "patchline.confidential-computing-report/v1" and
  .ok == true and
  .summary.enclaves == 2 and
  .summary.attested_enclaves == 2 and
  .summary.required_tee_kinds_met == 2 and
  .summary.key_release_policies == 2 and
  .summary.workloads == 2 and
  .summary.no_network_workloads == 2 and
  .summary.encrypted_inputs == 2 and
  .summary.private_outputs == 2 and
  .summary.redacted_public_outputs == 2 and
  .summary.aggregate_public_outputs == 2 and
  .summary.replay_proofs == 2 and
  .summary.verifier_reports == 2 and
  ([.enclaves[].quote.sha256 | select(test("^sha256:[0-9a-f]{64}$"))] | length) == 2
' "$OUT/safe/confidential-computing.json" > /dev/null

jq '
  .enclaves[0].measurement = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" |
  .enclaves[1].tee_kind = "tdx" |
  .enclaves[1].attestation_quote_path = "examples/confidential-computing/missing/sev-private-runner.quote.json" |
  .enclaves[1].verifier_report_path = "examples/confidential-computing/missing/sev-private-runner.verifier.json" |
  .key_release_policies[0].plaintext_export_allowed = true |
  .key_release_policies[0].requires_fresh_nonce = false |
  .key_release_policies[0].requires_reviewer_quorum = false |
  .key_release_policies[1].allowed_measurements = [] |
  .workloads[0].encrypted_input_paths = ["examples/confidential-computing/missing/rails-private-corpus.age"] |
  .workloads[0].output_manifest_sha256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000" |
  .workloads[0].network_egress_allowed = true |
  .workloads[0].outputs_redacted = false |
  .workloads[0].aggregate_only = false |
  .workloads[1].key_policy_id = "sgx-policy" |
  .workloads[1].private_output_paths = [] |
  .workloads[1].replay_evidence_paths = []
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline confidential-computing \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad confidential-computing spec unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_required_tee_kind")) and
  (.counterexamples | any(.kind == "missing_attestation_quote")) and
  (.counterexamples | any(.kind == "missing_verifier_report")) and
  (.counterexamples | any(.kind == "attestation_measurement_mismatch")) and
  (.counterexamples | any(.kind == "measurement_not_allowlisted")) and
  (.counterexamples | any(.kind == "missing_measurement_allowlist")) and
  (.counterexamples | any(.kind == "plaintext_export_allowed")) and
  (.counterexamples | any(.kind == "missing_fresh_nonce")) and
  (.counterexamples | any(.kind == "missing_reviewer_quorum")) and
  (.counterexamples | any(.kind == "missing_encrypted_input")) and
  (.counterexamples | any(.kind == "missing_encrypted_inputs")) and
  (.counterexamples | any(.kind == "output_manifest_hash_mismatch")) and
  (.counterexamples | any(.kind == "network_egress_allowed")) and
  (.counterexamples | any(.kind == "public_output_not_redacted")) and
  (.counterexamples | any(.kind == "public_output_not_aggregate")) and
  (.counterexamples | any(.kind == "workload_policy_enclave_mismatch")) and
  (.counterexamples | any(.kind == "workload_measurement_not_allowed")) and
  (.counterexamples | any(.kind == "missing_private_output")) and
  (.counterexamples | any(.kind == "missing_replay_evidence"))
' "$OUT/bad/confidential-computing.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/confidential-computing.json")"
go run ./cmd/patchline confidential-computing \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/confidential-computing.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: confidential-computing report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/confidential-computing.json" --slurpfile bad "$OUT/bad/confidential-computing.json" '{
  version: "patchline.confidential-computing-gate-results/v1",
  enclaves: $safe[0].summary.enclaves,
  attested_enclaves: $safe[0].summary.attested_enclaves,
  workloads: $safe[0].summary.workloads,
  encrypted_inputs: $safe[0].summary.encrypted_inputs,
  replay_proofs: $safe[0].summary.replay_proofs,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "confidential computing gate passed: attested enclaves, encrypted inputs, redacted aggregates, private outputs, and replay evidence verified"
