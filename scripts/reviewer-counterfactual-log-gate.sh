#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/live-feedback-ingestion-gate.json}"
HISTORY="${2:-examples/reviewer-counterfactual-policy-history.json}"
OUT="${3:-results/generated/reviewer-counterfactual-log-gate}"

rm -rf "$OUT/ingest" "$OUT/log"
mkdir -p "$OUT"

jq -e '.version == "patchline.counterfactual-policy-history/v1" and (.policies | length) >= 3' "$HISTORY" > /dev/null

for phrase in "Reviewer-outcome counterfactual logs" "boundary_ambiguous" "make reviewer-counterfactual-log-gate"; do
  grep -F "$phrase" docs/reviewer-counterfactual-log.md > /dev/null
done
grep -F "make reviewer-counterfactual-log-gate" README.md > /dev/null

go test ./internal/feedback -run 'TestCounterfactual'
go test ./cmd/patchline -run 'TestFeedbackCounterfactualLogWritesPreviousReleaseRecommendations'

go run ./cmd/patchline feedback ingest "$SPEC" --out "$OUT/ingest" --json > "$OUT/ingest.stdout.json"
go run ./cmd/patchline feedback counterfactual-log \
  --feedback "$OUT/ingest/live-feedback.json" \
  --history "$HISTORY" \
  --out "$OUT/log" \
  --json > "$OUT/log.stdout.json"

test -s "$OUT/log/counterfactual-log.json"
test -s "$OUT/log/counterfactual-log.md"

jq -e '
  .version == "patchline.reviewer-counterfactual-log/v1" and
  .ok == true and
  .evidence_basis == "published_k_anonymous_groups_only" and
  .release_ordering == "input_order_oldest_to_newest" and
  .summary.published_groups == 2 and
  .summary.counterfactual_groups_compared == 4 and
  .summary.compared_records == 12 and
  .summary.confirmed_would_block == 3 and
  .summary.false_positive_would_block == 3 and
  .summary.false_positive_would_spare == 3 and
  .summary.confirmed_boundary_ambiguous == 3 and
  (.entries | any(.policy_release == "v0.8.0" and .detector == "sql.destructive-ddl" and .classification == "boundary_ambiguous")) and
  (.entries | any(.policy_release == "v0.9.0" and .detector == "orm.write-breadth" and .classification == "would_block_false_positive")) and
  .privacy.source_free == true and
  .privacy.raw_values_free == true and
  .privacy.identifier_free == true and
  .privacy.salt_emitted == false and
  .privacy.individual_outcomes_free == true
' "$OUT/log/counterfactual-log.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/log/counterfactual-log.json")"
go run ./cmd/patchline feedback counterfactual-log \
  --feedback "$OUT/ingest/live-feedback.json" \
  --history "$HISTORY" \
  --out "$OUT/log-repeat" \
  --json > "$OUT/log-repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/log-repeat/counterfactual-log.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: counterfactual log hash is not deterministic" >&2
  exit 1
fi

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|db/migrate|source_code|raw_evidence|finding_id|evidence_hash|team-alpha|local-secret-feedback-salt-2026' \
  "$OUT/log/counterfactual-log.json" "$OUT/log/counterfactual-log.md" \
  "$OUT/log-repeat/counterfactual-log.json"; then
  echo "FAIL: counterfactual log retained source, raw evidence, identifiers, or salt" >&2
  exit 1
fi

jq -n --slurpfile log "$OUT/log/counterfactual-log.json" '{
  version: "patchline.reviewer-counterfactual-log-gate-results/v1",
  compared_records: $log[0].summary.compared_records,
  boundary_ambiguous: $log[0].summary.boundary_ambiguous,
  deterministic_hash: $log[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "reviewer-counterfactual-log gate passed: previous-release recommendations reconstructed from source-free reviewer outcomes"
