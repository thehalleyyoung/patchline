#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/adopter-active-learning-gate.json}"
OUT="${2:-results/generated/adopter-active-learning-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.adopter-active-learning/v1" and (.claim | length) > 200 and (.cases | length) >= 5' "$SPEC" > /dev/null
for phrase in "adopter-local active-learning queue" "shareable: false" "make adopter-active-learning-gate"; do
  grep -F "$phrase" docs/adopter-active-learning.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestActiveLearning'
go test ./cmd/patchline -run 'TestFeedbackLiveLearningCommandsWriteReports'

go run ./cmd/patchline feedback active-learning-queue \
  --spec "$SPEC" \
  --out "$OUT" \
  --json > "$OUT/stdout.json"

test -s "$OUT/active-learning-queue.json"
test -s "$OUT/active-learning-local-queue.json"
test -s "$OUT/active-learning-aggregate.json"
test -s "$OUT/active-learning-queue.md"

jq -e '
  .version == "patchline.adopter-active-learning-report/v1" and
  .ok == true and
  .shareable == false and
  .local_queue.shareable == false and
  .shareable_aggregate.shareable == true and
  .summary.queued_cases == 3 and
  .summary.already_labeled_cases == 1 and
  .summary.below_threshold_cases == 1
' "$OUT/active-learning-queue.json" > /dev/null

jq -e '
  .shareable == true and
  .source_free == true and
  .queued_cases == 3 and
  (.by_detector | length) == 3 and
  ([.. | objects | keys[]? | select(. == "local_case_id")] | length) == 0
' "$OUT/active-learning-aggregate.json" > /dev/null

if ! grep -F '"local_case_id"' "$OUT/active-learning-local-queue.json" > /dev/null; then
  echo "FAIL: local queue did not retain local case handles for adopter-only review" >&2
  exit 1
fi

if grep -Eiq 'case-001|case-002|case-003|case-004|case-005|DROP TABLE|UPDATE accounts|diff --git|source_code|raw_evidence|finding_id|evidence_hash' \
  "$OUT/active-learning-aggregate.json"; then
  echo "FAIL: active-learning aggregate leaked local examples, source, or raw identifiers" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/active-learning-queue.json" --slurpfile a "$OUT/active-learning-aggregate.json" '{
  version: "patchline.adopter-active-learning-gate-results/v1",
  queued_cases: $r[0].summary.queued_cases,
  aggregate_hash: $a[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "adopter-active-learning gate passed: local examples stay local while aggregate uncertainty metrics are shareable"
