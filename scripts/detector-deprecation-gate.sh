#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FEEDBACK_SPEC="${1:-examples/live-feedback-ingestion-gate.json}"
SPEC="${2:-examples/detector-deprecation-gate.json}"
OUT="${3:-results/generated/detector-deprecation-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.detector-deprecation/v1" and
  .as_of_date == "2026-06-15" and
  .min_evidence == 3 and
  .min_precision_bp == 9000 and
  .max_average_burden_minutes == 12 and
  .min_notice_days == 30 and
  .min_reviewer_roles == 2 and
  .min_appeal_window_days == 14 and
  (.required_public_channels | sort) == (["public-roadmap","release-notes"] | sort) and
  (.detectors | length) == 2
' "$SPEC" > /dev/null

for phrase in "Transparent detector deprecation" "detector-deprecation" "make detector-deprecation-gate"; do
  grep -F "$phrase" docs/detector-deprecation.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestDetectorDeprecation'
go test ./cmd/patchline -run TestFeedbackLiveLearningCommandsWriteReports

go run ./cmd/patchline feedback ingest "$FEEDBACK_SPEC" \
  --out "$OUT/ingest" \
  --json > "$OUT/ingest.stdout.json"

go run ./cmd/patchline feedback detector-deprecation \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$SPEC" \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/detector-deprecation.json"
test -s "$OUT/safe/detector-deprecation.md"

jq -e '
  .version == "patchline.detector-deprecation-report/v1" and
  .ok == true and
  .evidence_basis == "published_k_anonymous_groups_only" and
  .summary.detectors_evaluated == 2 and
  .summary.threshold_failures == 1 and
  .summary.ready_to_deprecate == 1 and
  .summary.retained == 1 and
  .summary.process_violations == 0 and
  (.detectors | any(.detector == "orm.write-breadth" and .status == "ready_to_deprecate" and .metrics.precision_bp == 0 and .notice_age_days >= 30)) and
  (.detectors | any(.detector == "sql.destructive-ddl" and .status == "retained_thresholds_met" and .metrics.precision_bp == 10000)) and
  .privacy.source_free == true and .privacy.raw_values_free == true and .privacy.identifier_free == true
' "$OUT/safe/detector-deprecation.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|db/migrate|source_code|raw_evidence|finding_id|evidence_hash|team-alpha|local-secret-feedback-salt-2026' \
  "$OUT/safe/detector-deprecation.json" "$OUT/safe/detector-deprecation.md"; then
  echo "FAIL: detector deprecation retained source, raw evidence, identifiers, or salt" >&2
  exit 1
fi

jq '.max_average_burden_minutes = 8' "$SPEC" > "$OUT/burden-threshold.json"
go run ./cmd/patchline feedback detector-deprecation \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$OUT/burden-threshold.json" \
  --out "$OUT/burden-threshold" \
  --json > "$OUT/burden-threshold.stdout.json"
jq -e '
  .ok == true and
  .summary.threshold_failures == 2 and
  .summary.ready_to_deprecate == 2 and
  (.detectors | any(.detector == "sql.destructive-ddl" and .status == "ready_to_deprecate" and .metrics.average_burden_minutes == 10))
' "$OUT/burden-threshold/detector-deprecation.json" > /dev/null

jq '
  (.detectors[] | select(.detector == "orm.write-breadth") | .public_notice_id) = "" |
  (.detectors[] | select(.detector == "orm.write-breadth") | .notice_opened_at) = "" |
  (.detectors[] | select(.detector == "orm.write-breadth") | .public_channels) = ["release-notes"] |
  (.detectors[] | select(.detector == "orm.write-breadth") | .reviewer_roles) = ["maintainer"]
' "$SPEC" > "$OUT/missing-process.json"
go run ./cmd/patchline feedback detector-deprecation \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$OUT/missing-process.json" \
  --out "$OUT/missing-process" \
  --json > "$OUT/missing-process.stdout.json"
jq -e '
  .ok == false and
  .summary.process_violations == 1 and
  (.detectors | any(.detector == "orm.write-breadth" and .status == "blocked_process_violation" and (.process_failures | index("public_notice")) and (.process_failures | index("notice_opened_at")) and (.process_failures | index("independent_reviewer_roles")) and (.missing_public_channels | index("public-roadmap"))))
' "$OUT/missing-process/detector-deprecation.json" > /dev/null

jq '(.detectors[] | select(.detector == "orm.write-breadth") | .notice_opened_at) = "2026-06-10"' "$SPEC" > "$OUT/fresh-notice.json"
go run ./cmd/patchline feedback detector-deprecation \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$OUT/fresh-notice.json" \
  --out "$OUT/fresh-notice" \
  --json > "$OUT/fresh-notice.stdout.json"
jq -e '
  .ok == true and
  (.detectors | any(.detector == "orm.write-breadth" and .status == "notice_open_in_review" and .notice_age_days == 5 and ((.process_failures // []) | length) == 0))
' "$OUT/fresh-notice/detector-deprecation.json" > /dev/null

jq '
  .min_notice_days = 7 |
  (.detectors[] | select(.detector == "orm.write-breadth") | .notice_opened_at) = "2026-06-01" |
  (.detectors[] | select(.detector == "orm.write-breadth") | .appeal_window_days) = 21
' "$SPEC" > "$OUT/appeal-open.json"
go run ./cmd/patchline feedback detector-deprecation \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$OUT/appeal-open.json" \
  --out "$OUT/appeal-open" \
  --json > "$OUT/appeal-open.stdout.json"
jq -e '
  .ok == true and
  (.detectors | any(.detector == "orm.write-breadth" and .status == "notice_open_in_review" and .notice_age_days == 14 and (.transparency_gates[] | select(.name == "notice_age_days" and .required == 21))))
' "$OUT/appeal-open/detector-deprecation.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/detector-deprecation.json")"
go run ./cmd/patchline feedback detector-deprecation \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$SPEC" \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/detector-deprecation.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: detector deprecation report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/detector-deprecation.json" --slurpfile burden "$OUT/burden-threshold/detector-deprecation.json" --slurpfile missing "$OUT/missing-process/detector-deprecation.json" --slurpfile fresh "$OUT/fresh-notice/detector-deprecation.json" --slurpfile appeal "$OUT/appeal-open/detector-deprecation.json" '{
  version: "patchline.detector-deprecation-gate-results/v1",
  ready_to_deprecate: $safe[0].summary.ready_to_deprecate,
  burden_ready_to_deprecate: $burden[0].summary.ready_to_deprecate,
  missing_process_violations: $missing[0].summary.process_violations,
  fresh_notice_status: ($fresh[0].detectors[] | select(.detector == "orm.write-breadth") | .status),
  appeal_window_status: ($appeal[0].detectors[] | select(.detector == "orm.write-breadth") | .status),
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "detector-deprecation gate passed: failing precision or burden thresholds require public notice, independent review, appeal time, and migration guidance"
