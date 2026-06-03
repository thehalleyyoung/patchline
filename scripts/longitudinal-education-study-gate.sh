#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/longitudinal-education-study.json}"
OUT="${2:-results/generated/longitudinal-education-study-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.longitudinal-education-study/v1" and
  (.claim | length) > 250 and
  (.hazards | length) == 3 and
  (.cohorts | length) == 2 and
  (.criteria.min_followup_months == 6)
' "$SPEC" > /dev/null

for phrase in "Longitudinal education study" "make longitudinal-education-study-gate"; do
  grep -F "$phrase" docs/longitudinal-education-study.md README.md > /dev/null
done

go test ./internal/education -run 'TestBuildLongitudinalStudyReport|TestReadLongitudinalStudySpec|TestWriteLongitudinalStudyArtifacts'
go test ./cmd/patchline -run TestLongitudinalEducationStudyCommandWritesReports

go run ./cmd/patchline longitudinal-education-study \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/longitudinal-education-study.json"
test -s "$OUT/safe/longitudinal-education-study.md"

jq -e '
  .version == "patchline.longitudinal-education-study-report/v1" and
  .ok == true and
  .summary.cohorts == 2 and
  .summary.trained_cohorts == 1 and
  .summary.control_cohorts == 1 and
  .summary.real_hazards == 3 and
  .summary.held_out_hazards == 3 and
  .summary.gate_backed_hazards == 3 and
  .summary.delayed_followup_month == 6 and
  .summary.trained_followup_detection_rate == 100 and
  .summary.control_followup_detection_rate == 33.33 and
  .summary.retention_lift_points == 66.67 and
  ([.hazards[].evidence[] | select(.sha256 | length == 64)] | length) == 6
' "$OUT/safe/longitudinal-education-study.json" > /dev/null

jq '
  .protocol.blind_review = false |
  .protocol.followup_months = [0, 1] |
  .cohorts = .cohorts[:1] |
  .hazards[0].gate = "missing-gate" |
  .hazards[0].reproduce_commands = [] |
  .observations = .observations[:6] |
  (.observations[] | select(.timepoint_month == 6) | .timepoint_month) = 1 |
  (.observations[] | select(.detected == true) | .commands) = [] |
  (.observations[] | select(.detected == true) | .evidence_citations) = []
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline longitudinal-education-study \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_control_cohort")) and
  (.counterexamples | any(.kind == "blind_protocol_missing")) and
  (.counterexamples | any(.kind == "hazard_unbacked")) and
  (.counterexamples | any(.kind == "non_reproducible_hazard")) and
  (.counterexamples | any(.kind == "missing_delayed_followup")) and
  (.counterexamples | any(.kind == "uncited_detection")) and
  (.counterexamples | any(.kind == "missing_gate_command"))
' "$OUT/bad/longitudinal-education-study.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/longitudinal-education-study.json")"
go run ./cmd/patchline longitudinal-education-study \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/longitudinal-education-study.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: longitudinal education study hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/longitudinal-education-study.json" --slurpfile bad "$OUT/bad/longitudinal-education-study.json" '{
  version: "patchline.longitudinal-education-study-gate-results/v1",
  cohorts: $safe[0].summary.cohorts,
  real_hazards: $safe[0].summary.real_hazards,
  delayed_followup_month: $safe[0].summary.delayed_followup_month,
  retention_lift_points: $safe[0].summary.retention_lift_points,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "longitudinal education study gate passed: delayed reviewer-retention lift is evidence-cited, gate-backed, and negative controls fail"
