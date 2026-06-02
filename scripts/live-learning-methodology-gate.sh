#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/live-learning-methodology-gate.json}"
OUT="${2:-results/generated/live-learning-methodology-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.live-learning-methodology/v1" and (.claim | length) > 200 and (.experiments | length) >= 2 and (.gate_evidence | length) >= 3' "$SPEC" > /dev/null
for phrase in "live-learning methodology report" "over-reliance" "make live-learning-methodology-gate"; do
  grep -F "$phrase" docs/live-learning-methodology.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestMethodologyReport|TestLiveLearningCanonical'
go test ./cmd/patchline -run 'TestFeedbackLiveLearningCommandsWriteReports'

go run ./cmd/patchline feedback methodology-report \
  --spec "$SPEC" \
  --out "$OUT" \
  --json > "$OUT/stdout.json"

test -s "$OUT/live-learning-methodology.json"
test -s "$OUT/live-learning-methodology.md"

jq -e '
  .version == "patchline.live-learning-methodology-report/v1" and
  .ok == true and
  .summary.experiments == 2 and
  .summary.recall_improved == 2 and
  .summary.overreliance_not_increased == 2 and
  .summary.linked_gate_evidence >= 3 and
  (.experiments | all(.recall_improved == true and .overreliance_not_increased == true)) and
  .privacy.source_free == true
' "$OUT/live-learning-methodology.json" > /dev/null

jq '(.experiments[0].live_learning_overreliance_bp) = (.experiments[0].baseline_overreliance_bp + 100)' "$SPEC" > "$OUT/methodology-negative.json"
go run ./cmd/patchline feedback methodology-report \
  --spec "$OUT/methodology-negative.json" \
  --out "$OUT/negative" \
  --json > "$OUT/negative.stdout.json"
jq -e '.ok == false and (.experiments | any(.overreliance_not_increased == false))' "$OUT/negative/live-learning-methodology.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|source_code|raw_evidence|finding_id|evidence_hash' \
  "$OUT/live-learning-methodology.json" "$OUT/live-learning-methodology.md"; then
  echo "FAIL: methodology output retained source or raw identifiers" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/live-learning-methodology.json" '{
  version: "patchline.live-learning-methodology-gate-results/v1",
  experiments: $r[0].summary.experiments,
  overreliance_negative_control: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "live-learning-methodology gate passed: recall improves without increasing over-reliance"
