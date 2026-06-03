#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/supply-chain-compromise-sim.json}"
OUT="${2:-results/generated/supply-chain-compromise-sim-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.supply-chain-compromise-simulation/v1" and
  (.claim | length) > 400 and
  ([.criteria.required_attack_kinds[]] | sort) == ["dependency_poisoning","forged_release_metadata","malicious_archive"] and
  (.simulations | length) == 3 and
  (.simulations | map(.kind) | sort) == ["dependency_poisoning","forged_release_metadata","malicious_archive"] and
  .criteria.require_detection == true and
  .criteria.require_rejection == true and
  .criteria.require_quarantine == true and
  .criteria.require_dependency_lock_integrity == true and
  .criteria.require_archive_entry_safety == true and
  .criteria.require_release_metadata_integrity == true and
  .criteria.require_signature_or_certificate_log == true
' "$SPEC" > /dev/null

for phrase in "Supply-chain compromise simulations" "dependency poisoning" "malicious archives" "forged release metadata" "make supply-chain-compromise-sim-gate"; do
  grep -F "$phrase" docs/supply-chain-compromise-simulations.md README.md > /dev/null
done

go test ./internal/supplychainsim -count=1
go test ./cmd/patchline -run TestSupplyChainCompromiseSimulationCommandWritesReports -count=1

go run ./cmd/patchline supply-chain simulate \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/supply-chain-sim.json"
test -s "$OUT/safe/supply-chain-sim.md"

jq -e '
  .version == "patchline.supply-chain-compromise-simulation-report/v1" and
  .ok == true and
  .summary.simulations == 3 and
  .summary.dependency_poisoning == 1 and
  .summary.malicious_archives == 1 and
  .summary.forged_release_metadata == 1 and
  .summary.detected_attacks == 3 and
  .summary.rejected_attacks == 3 and
  .summary.quarantined_attacks == 3 and
  .summary.attack_signals >= 15 and
  .summary.counterexamples == 0 and
  ([.simulations[].signals[].kind] | index("dependency_source_mismatch") != null) and
  ([.simulations[].signals[].kind] | index("dependency_lockfile_hash_mismatch") != null) and
  ([.simulations[].signals[].kind] | index("archive_entry_path_escape") != null) and
  ([.simulations[].signals[].kind] | index("archive_symlink_escape") != null) and
  ([.simulations[].signals[].kind] | index("archive_unexpected_executable_payload") != null) and
  ([.simulations[].signals[].kind] | index("release_metadata_digest_mismatch") != null) and
  ([.simulations[].signals[].kind] | index("release_ref_mismatch") != null)
' "$OUT/safe/supply-chain-sim.json" > /dev/null

jq '
  .simulations |= map(
    .detected = false |
    .rejected = false |
    .quarantined = false |
    if .kind == "dependency_poisoning" then
      .dependency.signature_path = ""
    elif .kind == "malicious_archive" then
      .archive.signature_path = ""
    elif .kind == "forged_release_metadata" then
      .release_metadata.signature_path = "" |
      .release_metadata.certificate_log_path = ""
    else
      .
    end
  )
' "$SPEC" > "$OUT/bad-spec.json"

set +e
go run ./cmd/patchline supply-chain simulate \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "FAIL: bad supply-chain simulation unexpectedly exited successfully" >&2
  exit 1
fi

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "dependency_source_mismatch_not_detected")) and
  (.counterexamples | any(.kind == "dependency_hash_mismatch_not_rejected")) and
  (.counterexamples | any(.kind == "dependency_lockfile_hash_mismatch_not_quarantined")) and
  (.counterexamples | any(.kind == "missing_dependency_signature")) and
  (.counterexamples | any(.kind == "unallowlisted_transitive_dependency_not_rejected")) and
  (.counterexamples | any(.kind == "archive_entry_path_escape_not_detected")) and
  (.counterexamples | any(.kind == "archive_symlink_escape_not_rejected")) and
  (.counterexamples | any(.kind == "archive_unexpected_executable_payload_not_quarantined")) and
  (.counterexamples | any(.kind == "missing_archive_signature")) and
  (.counterexamples | any(.kind == "release_metadata_digest_mismatch_not_detected")) and
  (.counterexamples | any(.kind == "release_manifest_hash_mismatch_not_rejected")) and
  (.counterexamples | any(.kind == "release_ref_mismatch_not_quarantined")) and
  (.counterexamples | any(.kind == "missing_release_signature")) and
  (.counterexamples | any(.kind == "missing_release_certificate_log"))
' "$OUT/bad/supply-chain-sim.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/supply-chain-sim.json")"
go run ./cmd/patchline supply-chain simulate \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/supply-chain-sim.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: supply-chain simulation report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/supply-chain-sim.json" --slurpfile bad "$OUT/bad/supply-chain-sim.json" '{
  version: "patchline.supply-chain-compromise-sim-gate-results/v1",
  simulations: $safe[0].summary.simulations,
  attack_signals: $safe[0].summary.attack_signals,
  detected_attacks: $safe[0].summary.detected_attacks,
  rejected_attacks: $safe[0].summary.rejected_attacks,
  quarantined_attacks: $safe[0].summary.quarantined_attacks,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "supply-chain compromise simulation gate passed: $(jq '.attack_signals' "$OUT/gate-summary.json") signals across dependency poisoning, malicious archive, and forged release metadata simulations"
