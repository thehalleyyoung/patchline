#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FEEDBACK_SPEC="${1:-examples/live-feedback-ingestion-gate.json}"
SPEC="${2:-examples/live-calibration-monitor-gate.json}"
OUT="${3:-results/generated/live-calibration-monitor-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.live-calibration-monitor/v1" and (.claim | length) > 200 and .pre_registered_tolerance_bp == 2000' "$SPEC" > /dev/null
for phrase in "live calibration monitor" "pre-registered tolerance" "make live-calibration-monitor-gate"; do
  grep -F "$phrase" docs/live-calibration-monitor.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestCalibrationMonitor'
go test ./cmd/patchline -run 'TestFeedbackLiveLearningCommandsWriteReports'

go run ./cmd/patchline feedback ingest "$FEEDBACK_SPEC" --out "$OUT/ingest" --json > "$OUT/ingest.stdout.json"
go run ./cmd/patchline feedback calibration-monitor \
  --feedback "$OUT/ingest/live-feedback.json" \
  --spec "$SPEC" \
  --out "$OUT/monitor" \
  --json > "$OUT/monitor.stdout.json"

test -s "$OUT/monitor/calibration-monitor.json"
test -s "$OUT/monitor/calibration-monitor.md"

jq -e '
  .version == "patchline.live-calibration-monitor-report/v1" and
  .ok == true and
  .evidence_basis == "published_k_anonymous_groups_only" and
  .summary.deciles_evaluated == 2 and
  .summary.alerts == 1 and
  (.alerts | any(.detector == "orm.write-breadth" and .drift_bp > .tolerance_bp and .severity == "critical")) and
  .privacy.source_free == true
' "$OUT/monitor/calibration-monitor.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|db/migrate|source_code|raw_evidence|finding_id|evidence_hash|team-alpha|local-secret-feedback-salt-2026' \
  "$OUT/monitor/calibration-monitor.json" "$OUT/monitor/calibration-monitor.md"; then
  echo "FAIL: calibration monitor retained source, raw evidence, identifiers, or salt" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/monitor/calibration-monitor.json" '{
  version: "patchline.live-calibration-monitor-gate-results/v1",
  alerts: $r[0].summary.alerts,
  tolerance_bp: $r[0].summary.tolerance_bp,
  verified: true
}' > "$OUT/gate-summary.json"

echo "live-calibration-monitor gate passed: decile drift beyond pre-registered tolerance alerts"
