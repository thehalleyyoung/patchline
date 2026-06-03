#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/practitioner-certification-exam.json}"
OUT="${2:-results/generated/practitioner-certification-exam-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.practitioner-certification/v1" and
  (.claim | length) > 200 and
  (.scenarios | length) == 3 and
  (.attempts | length) == 3
' "$SPEC" > /dev/null

for phrase in "Practitioner certification" "make practitioner-certification-exam-gate"; do
  grep -F "$phrase" docs/practitioner-certification-exam.md README.md > /dev/null
done

go test ./internal/practitionercertification -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestPractitionerCertificationCommandWritesReports

go run ./cmd/patchline practitioner-certification \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/practitioner-certification.json"
test -s "$OUT/safe/practitioner-certification.md"

jq -e '
  .version == "patchline.practitioner-certification-report/v1" and
  .ok == true and
  .summary.scenarios == 3 and
  .summary.gate_backed_scenarios == 3 and
  .summary.total_possible_points == 30 and
  .summary.passed_candidates == 1 and
  .candidates[0].score_pct == 100 and
  ([.scenarios[].evidence[] | select(.sha256 | length == 64)] | length) == 6
' "$OUT/safe/practitioner-certification.json" > /dev/null

jq '
  (.attempts[] | select(.scenario_id == "canary-validation-review") | .decision) = "approve_without_hash_review" |
  (.attempts[] | select(.scenario_id == "canary-validation-review") | .concepts) = ["row-count-preservation"] |
  (.attempts[] | select(.scenario_id == "canary-validation-review") | .commands) = []
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline practitioner-certification \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "decision_mismatch")) and
  (.counterexamples | any(.kind == "rubric_miss")) and
  (.counterexamples | any(.kind == "missing_reproducible_command")) and
  (.counterexamples | any(.kind == "candidate_failed"))
' "$OUT/bad/practitioner-certification.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/practitioner-certification.json")"
go run ./cmd/patchline practitioner-certification \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/practitioner-certification.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: practitioner certification hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/practitioner-certification.json" --slurpfile bad "$OUT/bad/practitioner-certification.json" '{
  version: "patchline.practitioner-certification-gate-results/v1",
  scenarios: $safe[0].summary.scenarios,
  gate_backed_scenarios: $safe[0].summary.gate_backed_scenarios,
  total_possible_points: $safe[0].summary.total_possible_points,
  passing_score_pct: $safe[0].criteria.passing_score_pct,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "practitioner certification exam gate passed: hands-on scenarios are gate-backed, reproducibly graded, and negative controls fail"
