#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/open-textbook-companion.json}"
OUT="${2:-results/generated/open-textbook-companion-gate}"
TEXTBOOK_OUT="results/generated/textbook-companion"

rm -rf "$OUT" "$TEXTBOOK_OUT"
mkdir -p "$OUT" "$TEXTBOOK_OUT"

jq -e '
  .version == "patchline.open-textbook-companion/v1" and
  (.claim | length) > 300 and
  (.chapters | length) == 3 and
  ([.chapters[].id] | sort) == ["classroom-labs","localized-lessons","reviewer-skills"] and
  .criteria.require_executable_notebook == true and
  .criteria.require_reproducible_commands == true and
  .criteria.require_generated_artifacts == true and
  .criteria.require_negative_control == true
' "$SPEC" > /dev/null

for phrase in "Open textbook companion" "make open-textbook-companion-gate" "executable notebooks"; do
  grep -F "$phrase" docs/open-textbook-companion.md README.md > /dev/null
done

go test ./internal/education -run 'TestBuildTextbookCompanionReport|TestReadTextbookCompanionSpec|TestWriteTextbookCompanionArtifacts' -count=1
go test ./cmd/patchline -run TestOpenTextbookCompanionCommandWritesReports -count=1

go run ./cmd/patchline classroom-lab-kits \
  --spec examples/classroom-lab-kits.json \
  --root . \
  --out "$TEXTBOOK_OUT/classroom-lab-kits" \
  --json > "$OUT/classroom-lab-kits.stdout.json"

go run ./cmd/patchline skills-taxonomy \
  --spec examples/skills-taxonomy.json \
  --root . \
  --out "$TEXTBOOK_OUT/skills-taxonomy" \
  --json > "$OUT/skills-taxonomy.stdout.json"

go run ./cmd/patchline localized-teaching-examples \
  --spec examples/localized-teaching-examples.json \
  --root . \
  --out "$TEXTBOOK_OUT/localized-teaching-examples" \
  --json > "$OUT/localized-teaching-examples.stdout.json"

go run ./cmd/patchline open-textbook-companion \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/open-textbook-companion.json"
test -s "$OUT/safe/open-textbook-companion.md"

jq -e '
  .version == "patchline.open-textbook-companion-report/v1" and
  .ok == true and
  .summary.chapters == 3 and
  .summary.notebooks == 3 and
  .summary.executable_notebooks == 3 and
  .summary.teaching_examples == 3 and
  .summary.commands == 3 and
  .summary.evidence_artifacts == 9 and
  .summary.generated_artifacts == 6 and
  .summary.negative_controls == 3 and
  ([.chapters[].notebook_reports[].generated_artifacts[] | select(.sha256 | length == 64)] | length) == 6
' "$OUT/safe/open-textbook-companion.json" > /dev/null

jq '
  .chapters = .chapters[:2] |
  .chapters[0].notebooks[0].path = "examples/textbook-companion/missing.ipynb" |
  .chapters[0].notebooks[0].execute_commands = ["make missing-textbook-command"] |
  .chapters[0].notebooks[0].teaching_examples = [] |
  .chapters[0].notebooks[0].evidence_paths = ["docs/missing-textbook-evidence.md"] |
  .chapters[0].notebooks[0].expected_artifacts = [
    "results/generated/textbook-companion/missing/missing.json",
    "results/generated/textbook-companion/missing/missing.md"
  ] |
  .chapters[0].notebooks[0].negative_controls = []
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline open-textbook-companion \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "missing_required_chapter")) and
  (.counterexamples | any(.kind == "missing_notebook")) and
  (.counterexamples | any(.kind == "missing_executable_cell")) and
  (.counterexamples | any(.kind == "insufficient_teaching_examples")) and
  (.counterexamples | any(.kind == "missing_evidence")) and
  (.counterexamples | any(.kind == "missing_regenerated_artifact")) and
  (.counterexamples | any(.kind == "insufficient_generated_artifacts")) and
  (.counterexamples | any(.kind == "missing_negative_control"))
' "$OUT/bad/open-textbook-companion.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/open-textbook-companion.json")"
go run ./cmd/patchline open-textbook-companion \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/open-textbook-companion.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: open textbook companion hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/open-textbook-companion.json" --slurpfile bad "$OUT/bad/open-textbook-companion.json" '{
  version: "patchline.open-textbook-companion-gate-results/v1",
  chapters: $safe[0].summary.chapters,
  notebooks: $safe[0].summary.notebooks,
  executable_notebooks: $safe[0].summary.executable_notebooks,
  teaching_examples: $safe[0].summary.teaching_examples,
  generated_artifacts: $safe[0].summary.generated_artifacts,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "open textbook companion gate passed: executable notebooks regenerate teaching examples and stale or incomplete material fails"
