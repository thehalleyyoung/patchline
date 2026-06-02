#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/live-feedback-ingestion-gate.json}"
POLICY="${2:-examples/drift-threshold-policy.json}"
OUT="${3:-results/generated/drift-threshold-update-gate}"

rm -rf "$OUT/ingest" "$OUT/advisory" "$OUT/stale" "$OUT/gated"
mkdir -p "$OUT"

for phrase in "Drift-aware threshold updates" "blocked_without_gate" "make drift-threshold-update-gate"; do
  grep -F "$phrase" docs/drift-threshold-update.md > /dev/null
done
grep -F "make drift-threshold-update-gate" README.md > /dev/null

go test ./internal/feedback -run 'TestThresholdUpdate'
go test ./cmd/patchline -run 'TestFeedbackThresholdUpdateRequiresBoundGateForCandidatePolicy'

policy_before="$(shasum -a 256 "$POLICY" | awk '{print $1}')"

go run ./cmd/patchline feedback ingest "$SPEC" --out "$OUT/ingest" --json > "$OUT/ingest.stdout.json"
go run ./cmd/patchline feedback threshold-update \
  --feedback "$OUT/ingest/live-feedback.json" \
  --policy "$POLICY" \
  --out "$OUT/advisory" \
  --json > "$OUT/advisory.stdout.json"

test -s "$OUT/advisory/threshold-update.json"
test -s "$OUT/advisory/threshold-update.md"
test ! -e "$OUT/advisory/candidate-threshold-policy.json"

jq -e '
  .version == "patchline.threshold-update/v1" and
  .ok == true and
  .policy_change_allowed == false and
  .blocking_policy_changed == false and
  .evidence_basis == "published_k_anonymous_groups_only" and
  .gate.required == true and
  .gate.provided == false and
  .gate.reason == "missing_policy_gate" and
  .summary.recommendations >= 1 and
  .summary.blocked_without_gate == .summary.recommendations and
  (.recommendations | any(.detector == "orm.write-breadth" and .direction == "raise" and .apply_status == "blocked_without_gate"))
' "$OUT/advisory/threshold-update.json" > /dev/null

policy_hash="$(jq -r '.policy_hash' "$OUT/advisory/threshold-update.json")"
feedback_hash="$(jq -r '.feedback_hash' "$OUT/advisory/threshold-update.json")"

jq -n \
  --arg feedback_hash "$feedback_hash" \
  '{
    version: "patchline.threshold-policy-gate/v1",
    gate: "drift-threshold-update-gate",
    ok: true,
    policy_hash: "stale-policy-hash",
    feedback_hash: $feedback_hash,
    allows_blocking_policy_change: true
  }' > "$OUT/stale-gate.json"

go run ./cmd/patchline feedback threshold-update \
  --feedback "$OUT/ingest/live-feedback.json" \
  --policy "$POLICY" \
  --gate "$OUT/stale-gate.json" \
  --out "$OUT/stale" \
  --json > "$OUT/stale.stdout.json"

test ! -e "$OUT/stale/candidate-threshold-policy.json"
jq -e '
  .policy_change_allowed == false and
  .blocking_policy_changed == false and
  .gate.provided == true and
  .gate.policy_hash_matches == false and
  .gate.feedback_hash_matches == true and
  .gate.ok == false
' "$OUT/stale/threshold-update.json" > /dev/null

jq -n \
  --arg policy_hash "$policy_hash" \
  --arg feedback_hash "$feedback_hash" \
  '{
    version: "patchline.threshold-policy-gate/v1",
    gate: "drift-threshold-update-gate",
    ok: true,
    policy_hash: $policy_hash,
    feedback_hash: $feedback_hash,
    allows_blocking_policy_change: true,
    reviewer: "artifact-reviewer",
    reproduction_command: "make drift-threshold-update-gate"
  }' > "$OUT/valid-gate.json"

go run ./cmd/patchline feedback threshold-update \
  --feedback "$OUT/ingest/live-feedback.json" \
  --policy "$POLICY" \
  --gate "$OUT/valid-gate.json" \
  --out "$OUT/gated" \
  --json > "$OUT/gated.stdout.json"

test -s "$OUT/gated/candidate-threshold-policy.json"
jq -e '
  .policy_change_allowed == true and
  .blocking_policy_changed == false and
  .gate.ok == true and
  .gate.policy_hash_matches == true and
  .gate.feedback_hash_matches == true and
  .summary.candidate_changes == .summary.recommendations and
  (.recommendations | any(.detector == "orm.write-breadth" and .apply_status == "candidate_requires_review"))
' "$OUT/gated/threshold-update.json" > /dev/null
jq -e '
  .version == "patchline.threshold-policy/v1" and
  (.thresholds | any(.detector == "orm.write-breadth" and .blocking_threshold == 0.8))
' "$OUT/gated/candidate-threshold-policy.json" > /dev/null

policy_after="$(shasum -a 256 "$POLICY" | awk '{print $1}')"
if [[ "$policy_before" != "$policy_after" ]]; then
  echo "FAIL: threshold updater mutated the input blocking policy" >&2
  exit 1
fi

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|db/migrate|source_code|raw_evidence|finding_id|evidence_hash|team-alpha|local-secret-feedback-salt-2026' \
  "$OUT/advisory/threshold-update.json" "$OUT/advisory/threshold-update.md" \
  "$OUT/stale/threshold-update.json" "$OUT/gated/threshold-update.json" \
  "$OUT/gated/candidate-threshold-policy.json"; then
  echo "FAIL: threshold update retained source, raw evidence, identifiers, or salt" >&2
  exit 1
fi

jq -n --slurpfile advisory "$OUT/advisory/threshold-update.json" --slurpfile gated "$OUT/gated/threshold-update.json" '{
  version: "patchline.drift-threshold-update-gate-results/v1",
  advisory_recommendations: $advisory[0].summary.recommendations,
  gated_candidate_changes: $gated[0].summary.candidate_changes,
  stale_gate_rejected: true,
  input_policy_unchanged: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "drift-threshold-update gate passed: advisory drift suggestions require a hash-bound policy gate"
