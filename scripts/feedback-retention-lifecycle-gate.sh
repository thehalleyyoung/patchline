#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/feedback-retention-lifecycle-gate.json}"
OUT="${2:-results/generated/feedback-retention-lifecycle-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.feedback-retention-lifecycle/v1" and (.claim | length) > 200 and (.artifacts | length) >= 3' "$SPEC" > /dev/null
for phrase in "evidence-retention lifecycle" "fixed as-of date" "make feedback-retention-lifecycle-gate"; do
  grep -F "$phrase" docs/feedback-retention-lifecycle.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestRetentionLifecycle'
go test ./cmd/patchline -run 'TestFeedbackLiveLearningCommandsWriteReports'

go run ./cmd/patchline feedback retention-lifecycle \
  --spec "$SPEC" \
  --out "$OUT" \
  --json > "$OUT/stdout.json"

test -s "$OUT/retention-lifecycle.json"
test -s "$OUT/retention-lifecycle.md"

jq -e '
  .version == "patchline.feedback-retention-lifecycle-report/v1" and
  .ok == true and
  .as_of_date == "2026-06-02" and
  .summary.artifacts_evaluated == 3 and
  .summary.delete_required == 1 and
  .summary.anonymize_required == 1 and
  .summary.aggregate_retained == 1 and
  (.artifacts | any(.artifact_id == "raw-old" and .expected_action == "delete" and .compliant == true)) and
  .privacy.source_free == true
' "$OUT/retention-lifecycle.json" > /dev/null

jq '(.artifacts[] | select(.artifact_id == "raw-old") | .observed_action) = "retain_local"' "$SPEC" > "$OUT/retention-violation.json"
go run ./cmd/patchline feedback retention-lifecycle \
  --spec "$OUT/retention-violation.json" \
  --out "$OUT/violation" \
  --json > "$OUT/violation.stdout.json"
jq -e '.ok == false and .summary.violations == 1' "$OUT/violation/retention-lifecycle.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|source_code|raw_evidence|finding_id|evidence_hash' \
  "$OUT/retention-lifecycle.json" "$OUT/retention-lifecycle.md"; then
  echo "FAIL: retention lifecycle output retained source or raw identifiers" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/retention-lifecycle.json" '{
  version: "patchline.feedback-retention-lifecycle-gate-results/v1",
  delete_required: $r[0].summary.delete_required,
  violation_negative_control: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "feedback-retention-lifecycle gate passed: feedback expires, anonymizes, or aggregates under deterministic policy"
