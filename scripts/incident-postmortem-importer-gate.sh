#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/incident-postmortem-import.json}"
OUT="${2:-results/generated/incident-postmortem-importer-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.incident-postmortem-import/v1" and
  .historical_suite == "historical-failures/suite.json" and
  (.include_cases | length) == 2 and
  .min_regressions >= 20
' "$SPEC" > /dev/null

for phrase in "Incident-postmortem importer" "incident-postmortem-import" "detector regression tests" "make incident-postmortem-importer-gate"; do
  grep -F "$phrase" docs/incident-postmortem-importer.md README.md > /dev/null
done

go test ./internal/historical -run 'TestRunHistoricalSuite'
go test ./internal/incidentpostmortem -run 'TestBuildReport|TestWriteArtifacts|TestDetectSignal|TestReadSpec'
go test ./cmd/patchline -run TestIncidentPostmortemImportCommandWritesReports

go run ./cmd/patchline incident-postmortem-import \
  --spec "$SPEC" \
  --out "$OUT/report" \
  --json > "$OUT/stdout.json"

test -s "$OUT/report/incident-postmortem-import.json"
test -s "$OUT/report/incident-postmortem-import.md"
test -s "$OUT/report/detector-regressions.json"
test -s "$OUT/report/generated-tests/incident_postmortem_regression_test.go"

jq -e '
  .version == "patchline.incident-postmortem-import-report/v1" and
  .ok == true and
  .summary.cases == 2 and
  .summary.source_observations >= 13 and
  .summary.regressions >= 20 and
  .summary.detectors >= 4 and
  .summary.failed == 0 and
  (.regressions | any(.detector_signal_id == "protected-primary-mutation" and .positive.detected == true and (.negatives | all(.detected == false)))) and
  (.regressions | any(.detector_signal_id == "missing-snapshot-rollback" and .positive.detected == true and (.negatives | all(.detected == false)))) and
  (.regressions | any(.detector_signal_id == "split-brain-conflicting-writes" and .positive.detected == true and (.negatives | length == 2 and all(.detected == false)))) and
  (.regressions | any(.detector_signal_id == "source-established-primary-data-loss" and .positive.detected == true))
' "$OUT/report/incident-postmortem-import.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/report/incident-postmortem-import.json")"
go run ./cmd/patchline incident-postmortem-import \
  --spec "$SPEC" \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/incident-postmortem-import.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: incident-postmortem importer hash is not deterministic" >&2
  exit 1
fi

go test "./$OUT/report/generated-tests"

jq -n --slurpfile report "$OUT/report/incident-postmortem-import.json" '{
  version: "patchline.incident-postmortem-importer-gate-results/v1",
  cases: $report[0].summary.cases,
  source_observations: $report[0].summary.source_observations,
  regressions: $report[0].summary.regressions,
  detectors: $report[0].summary.detectors,
  deterministic_hash: $report[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "incident-postmortem-importer gate passed: $(jq '.regressions' "$OUT/gate-summary.json") regression tests from public postmortem lessons"
