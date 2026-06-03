#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/skills-taxonomy.json}"
OUT="${2:-results/generated/skills-taxonomy-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.skills-taxonomy/v1" and
  (.claim | length) > 250 and
  (.hazard_classes | length) == 5 and
  (.criteria.required_audiences | sort) == ["app-developer","dba","engineering-manager","security-reviewer","sre"] and
  .criteria.min_concepts_per_hazard == 2 and
  .criteria.require_crosswalk == true
' "$SPEC" > /dev/null

for phrase in "Reviewer skills taxonomy" "make skills-taxonomy-gate" "hazard class"; do
  grep -F "$phrase" docs/skills-taxonomy.md README.md > /dev/null
done

go test ./internal/education -run 'TestBuildSkillsTaxonomyReport|TestReadSkillsTaxonomySpec|TestWriteSkillsTaxonomyArtifacts' -count=1
go test ./cmd/patchline -run TestSkillsTaxonomyCommandWritesReports -count=1

go run ./cmd/patchline skills-taxonomy \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/skills-taxonomy.json"
test -s "$OUT/safe/skills-taxonomy.md"

jq -e '
  .version == "patchline.skills-taxonomy-report/v1" and
  .ok == true and
  .summary.hazard_classes == 5 and
  .summary.concepts == 10 and
  .summary.reviewer_audiences == 5 and
  .summary.gate_backed_hazards == 5 and
  .summary.evidence_artifacts == 15 and
  .summary.negative_controls == 5 and
  .summary.crosswalk_entries == 10 and
  ([.hazard_classes[].evidence[] | select(.sha256 | length == 64)] | length) == 15
' "$OUT/safe/skills-taxonomy.json" > /dev/null

jq '
  .hazard_classes = .hazard_classes[:4] |
  .hazard_classes[0].concepts = [.hazard_classes[0].concepts[0]] |
  .hazard_classes[0].concepts[0].prerequisites = [] |
  .hazard_classes[0].concepts[0].assessment_prompt = "" |
  .hazard_classes[0].concepts[0].evidence_paths = [] |
  .hazard_classes[0].gates[0].name = "missing-gate" |
  .hazard_classes[0].gates[0].commands = [] |
  .hazard_classes[0].gates[0].evidence_paths = [] |
  .hazard_classes[0].gates[0].negative_controls = [] |
  .hazard_classes[0].related_tutorials = [] |
  .hazard_classes[0].related_certification_scenarios = [] |
  .hazard_classes[3].reviewer_audiences = []
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline skills-taxonomy \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "insufficient_hazard_classes")) and
  (.counterexamples | any(.kind == "missing_required_audience")) and
  (.counterexamples | any(.kind == "missing_reviewer_audience")) and
  (.counterexamples | any(.kind == "insufficient_concepts")) and
  (.counterexamples | any(.kind == "insufficient_prerequisites")) and
  (.counterexamples | any(.kind == "missing_assessment_prompt")) and
  (.counterexamples | any(.kind == "missing_concept_evidence")) and
  (.counterexamples | any(.kind == "missing_gate")) and
  (.counterexamples | any(.kind == "missing_reproducible_command")) and
  (.counterexamples | any(.kind == "missing_negative_control")) and
  (.counterexamples | any(.kind == "insufficient_hazard_evidence")) and
  (.counterexamples | any(.kind == "missing_tutorial_crosswalk")) and
  (.counterexamples | any(.kind == "missing_certification_crosswalk"))
' "$OUT/bad/skills-taxonomy.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/skills-taxonomy.json")"
go run ./cmd/patchline skills-taxonomy \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/skills-taxonomy.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: skills taxonomy hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/skills-taxonomy.json" --slurpfile bad "$OUT/bad/skills-taxonomy.json" '{
  version: "patchline.skills-taxonomy-gate-results/v1",
  hazard_classes: $safe[0].summary.hazard_classes,
  concepts: $safe[0].summary.concepts,
  reviewer_audiences: $safe[0].summary.reviewer_audiences,
  evidence_artifacts: $safe[0].summary.evidence_artifacts,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "skills taxonomy gate passed: hazard classes map to reviewer concepts, evidence, gates, negative controls, and crosswalks"
