#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/classroom-lab-kits.json}"
OUT="${2:-results/generated/classroom-lab-kits-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.classroom-lab-kit/v1" and
  (.claim | length) > 250 and
  (.courses | length) == 4 and
  ([.courses[].audience] | sort) == ["database","devops","programming-languages","software-engineering"]
' "$SPEC" > /dev/null

for phrase in "Classroom lab kits" "make classroom-lab-kits-gate" "instructor solution gate"; do
  grep -F "$phrase" docs/classroom-lab-kits.md README.md > /dev/null
done

go test ./internal/education -run 'TestBuildLabKitReport|TestReadLabKitSpec|TestWriteLabKitArtifacts'
go test ./cmd/patchline -run TestClassroomLabKitsCommandWritesReports

go run ./cmd/patchline classroom-lab-kits \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/classroom-lab-kits.json"
test -s "$OUT/safe/classroom-lab-kits.md"

jq -e '
  .version == "patchline.classroom-lab-kit-report/v1" and
  .ok == true and
  .summary.courses == 4 and
  .summary.labs == 4 and
  .summary.audiences_covered == 4 and
  .summary.gate_backed_labs == 4 and
  .summary.evidence_artifacts == 8 and
  .summary.negative_controls == 4 and
  ([.courses[].lab_reports[].evidence[] | select(.sha256 | length == 64)] | length) == 8
' "$OUT/safe/classroom-lab-kits.json" > /dev/null

jq '
  .courses = .courses[:3] |
  (.courses[0].labs[0].instructor_solution.gate) = "missing-gate" |
  (.courses[0].labs[0].instructor_solution.commands) = [] |
  (.courses[0].labs[0].negative_controls) = [] |
  (.courses[0].labs[0].evidence_paths) = ["docs/missing-lab-evidence.md"]
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline classroom-lab-kits \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_required_audience")) and
  (.counterexamples | any(.kind == "missing_instructor_solution_gate")) and
  (.counterexamples | any(.kind == "missing_reproducible_command")) and
  (.counterexamples | any(.kind == "missing_negative_control")) and
  (.counterexamples | any(.kind == "missing_evidence")) and
  (.counterexamples | any(.kind == "insufficient_lab_evidence"))
' "$OUT/bad/classroom-lab-kits.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/classroom-lab-kits.json")"
go run ./cmd/patchline classroom-lab-kits \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/classroom-lab-kits.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: classroom lab kit hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/classroom-lab-kits.json" --slurpfile bad "$OUT/bad/classroom-lab-kits.json" '{
  version: "patchline.classroom-lab-kits-gate-results/v1",
  courses: $safe[0].summary.courses,
  labs: $safe[0].summary.labs,
  audiences_covered: $safe[0].summary.audiences_covered,
  gate_backed_labs: $safe[0].summary.gate_backed_labs,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "classroom lab kits gate passed: four course tracks are instructor-gate-backed, evidence-hashed, and negative controls fail"
