#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/workforce-impact-study.json}"
OUT="${2:-results/generated/workforce-impact-study-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.workforce-impact-study/v1" and
  (.claim | length) > 350 and
  (.automations | length) == 2 and
  (.cohorts | length) == 2 and
  (.observations | length) == 8 and
  .criteria.min_observations_per_cohort_period == 2
' "$SPEC" > /dev/null

for path in $(jq -r '.automations[].evidence_paths[], .observations[].evidence_paths[]' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Workforce-impact study" "workforce-impact-study" "make workforce-impact-study-gate"; do
  grep -F "$phrase" docs/workforce-impact-study.md README.md > /dev/null
done

go test ./internal/education -run 'TestBuildWorkforceImpactReport|TestReadWorkforceImpactSpec|TestWriteWorkforceImpactArtifacts'
go test ./cmd/patchline -run TestWorkforceImpactStudyCommandWritesReports

go run ./cmd/patchline workforce-impact-study \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/workforce-impact-study.json"
test -s "$OUT/safe/workforce-impact-study.md"

jq -e '
  .version == "patchline.workforce-impact-study-report/v1" and
  .ok == true and
  .summary.cohorts == 2 and
  .summary.treated_cohorts == 1 and
  .summary.control_cohorts == 1 and
  .summary.automation_references == 2 and
  .summary.gate_backed_automations == 2 and
  .summary.ownership_diff_in_diff_points == 100 and
  .summary.escalation_diff_in_diff_points == 100 and
  .summary.learning_diff_in_diff_points == 23 and
  .summary.held_out_detection_diff_in_diff_points == 66.67 and
  .summary.treated_downstream_miss_rate_increase_points == 0 and
  ([.observations[].evidence[] | select(.sha256 | length == 64)] | length) >= 8
' "$OUT/safe/workforce-impact-study.json" > /dev/null

jq '
  .automations[0].gate = "missing-gate" |
  (.observations[] | select(.period == "post-automation") | .automation_refs) = [] |
  (.observations[] | select(.period == "post-automation") | .commands) = [] |
  (.observations[] | .evidence_paths) = [] |
  (.observations[] | select(.review_id == "treated-before-01") | .participant_id) = "alice@example.com" |
  (.observations[] | select(.cohort_id == "patchline-treated" and .period == "post-automation") | .downstream_misses) = 1 |
  (.observations[] | select(.cohort_id == "ordinary-control" and .period == "post-automation") | .owned_by_primary_team) = true |
  (.observations[] | select(.cohort_id == "patchline-treated" and .period == "post-automation") | .held_out_detections) = 1
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline workforce-impact-study \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_gate_reference")) and
  (.counterexamples | any(.kind == "missing_automation_reference")) and
  (.counterexamples | any(.kind == "missing_evidence")) and
  (.counterexamples | any(.kind == "pii_like_identifier")) and
  (.counterexamples | any(.kind == "confounded_by_secular_trend")) and
  (.counterexamples | any(.kind == "suppressed_escalation")) and
  (.counterexamples | any(.kind == "insufficient_heldout_detection_lift")) and
  (.counterexamples | any(.kind == "teaching_to_test"))
' "$OUT/bad/workforce-impact-study.json" > /dev/null

jq '
  .observations = [.observations[] | select(.review_id != "control-after-02")]
' "$SPEC" > "$OUT/missing-period-spec.json"

go run ./cmd/patchline workforce-impact-study \
  --spec "$OUT/missing-period-spec.json" \
  --root . \
  --out "$OUT/missing-period" \
  --json > "$OUT/missing-period.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_period_observations"))
' "$OUT/missing-period/workforce-impact-study.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/workforce-impact-study.json")"
go run ./cmd/patchline workforce-impact-study \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/workforce-impact-study.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: workforce impact study hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/workforce-impact-study.json" --slurpfile bad "$OUT/bad/workforce-impact-study.json" '{
  version: "patchline.workforce-impact-study-gate-results/v1",
  cohorts: $safe[0].summary.cohorts,
  observations: $safe[0].summary.observations,
  ownership_diff_in_diff_points: $safe[0].summary.ownership_diff_in_diff_points,
  escalation_diff_in_diff_points: $safe[0].summary.escalation_diff_in_diff_points,
  learning_diff_in_diff_points: $safe[0].summary.learning_diff_in_diff_points,
  held_out_detection_diff_in_diff_points: $safe[0].summary.held_out_detection_diff_in_diff_points,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "workforce impact study gate passed: ownership, escalation, and learning effects are difference-in-differences, evidence-hashed, and quality-guarded"
