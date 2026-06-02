#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/qualitative-notes-gate.json}"
OUT="${2:-results/generated/qualitative-notes-gate}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.qualitative-notes-gate/v1" and
  (.claim | length) > 160 and
  (.required_labels | length) == 4 and
  (.required_note_fields | length) >= 8 and
  .minimum_public_repos >= 8 and
  (.real_code | length) >= 8 and
  all(.real_code[]; (.repo | length) > 0 and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

for field in rubric notes label status confidence observation evidence coder_instruction maintainer_question recommended_decision false_positive_candidate false_negative_candidate proof_hole maintainer_decision; do
  grep -F "$field" docs/qualitative-coding-notes.md > /dev/null
done
grep -F "make qualitative-notes-gate" README.md > /dev/null

go test ./cmd/patchline -run TestRepoQualitativeNotesWriteCodingNotesFromAnalyses > "$OUT/go-test.log"

analysis_dirs=()
count="$(jq '.real_code | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".real_code[$i].id" "$SPEC")"
  repo="$(jq -r ".real_code[$i].repo" "$SPEC")"
  ref="$(jq -r ".real_code[$i].ref" "$SPEC")"
  subpath="$(jq -r ".real_code[$i].subpath" "$SPEC")"
  analysis="$OUT/analyses/$id"
  analysis_dirs+=("$analysis")
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline,propose,compare \
    --proposal-kind all \
    --budget files=3,lines=80,tokens=12000,changes=2 \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$i.json"
done

IFS=,
analyses="${analysis_dirs[*]}"
unset IFS

go run ./cmd/patchline repo qualitative-notes \
  --analyses "$analyses" \
  --out "$OUT/notes" \
  --json > "$OUT/notes-stdout.json"

min_repos="$(jq '.minimum_public_repos' "$SPEC")"
labels_json="$(jq -c '.required_labels' "$SPEC")"
jq -e --argjson min_repos "$min_repos" --argjson labels "$labels_json" '
  .version == "patchline.repo-qualitative-notes/v1" and
  .summary.analyses >= $min_repos and
  .summary.public_repos >= $min_repos and
  .summary.notes >= (.summary.public_repos * 4) and
  .summary.false_positive_notes > 0 and
  .summary.false_negative_notes > 0 and
  .summary.proof_hole_notes > 0 and
  .summary.maintainer_decision_notes > 0 and
  . as $report |
  all($labels[]; ($report.summary.by_label[.] // 0) > 0) and
  all(.notes[];
    (.id | length) > 0 and
    (.label | length) > 0 and
    (.status | length) > 0 and
    (.confidence | length) > 0 and
    (.observation | length) > 20 and
    (.evidence | length) > 0 and
    (.coder_instruction | length) > 20 and
    (.maintainer_question | length) > 20 and
    (.recommended_decision | length) > 10
  )
' "$OUT/notes/qualitative-notes.json" > /dev/null

for label in $(jq -r '.required_labels[]' "$SPEC"); do
  grep -F "$label" "$OUT/notes/qualitative-notes.md" > /dev/null
done
for repo in $(jq -r '.real_code[].repo' "$SPEC"); do
  grep -F "$repo" "$OUT/notes/qualitative-notes.md" > /dev/null
done
grep -F "qualitative coding notes" "$OUT/notes/qualitative-notes.md" > /dev/null
grep -F "maintainer question" "$OUT/notes/qualitative-notes.md" > /dev/null

jq -n \
  --slurpfile notes "$OUT/notes/qualitative-notes.json" \
  '{
    version:"patchline.qualitative-notes-gate-results/v1",
    analyses:$notes[0].summary.analyses,
    public_repos:$notes[0].summary.public_repos,
    notes:$notes[0].summary.notes,
    false_positive_notes:$notes[0].summary.false_positive_notes,
    false_negative_notes:$notes[0].summary.false_negative_notes,
    proof_hole_notes:$notes[0].summary.proof_hole_notes,
    maintainer_decision_notes:$notes[0].summary.maintainer_decision_notes,
    hash:$notes[0].hash,
    verified:true
  }' > "$OUT/summary.json"

jq -e --argjson min_repos "$min_repos" '.verified == true and .public_repos >= $min_repos and .false_positive_notes > 0 and .false_negative_notes > 0 and .proof_hole_notes > 0 and .maintainer_decision_notes > 0' "$OUT/summary.json" > /dev/null

echo "qualitative notes gate passed: notes $(jq '.notes' "$OUT/summary.json"), public repos $(jq '.public_repos' "$OUT/summary.json")"
