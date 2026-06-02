#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/human-trust-regression-gate.json}"
OUT="${2:-results/generated/human-trust-regression-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.human-trust-regression/v1" and (.claim | length) > 200 and .baseline.release == "v1.0.0"' "$SPEC" > /dev/null
for phrase in "human-trust regression suite" "over-reliance" "make human-trust-regression-gate"; do
  grep -F "$phrase" docs/human-trust-regression.md README.md > /dev/null
done

go test ./internal/feedback -run 'TestTrustRegression'
go test ./cmd/patchline -run 'TestFeedbackLiveLearningCommandsWriteReports'

go run ./cmd/patchline feedback trust-regression \
  --spec "$SPEC" \
  --out "$OUT" \
  --json > "$OUT/stdout.json"

test -s "$OUT/trust-regression.json"
test -s "$OUT/trust-regression.md"

jq -e '
  .version == "patchline.human-trust-regression-report/v1" and
  .ok == true and
  .summary.checks == 5 and
  .summary.failed_checks == 0 and
  (.checks | all(.passed == true)) and
  .privacy.source_free == true
' "$OUT/trust-regression.json" > /dev/null

jq '.current.explanation_faithfulness_bp = 7800 | .current.overreliance_rate_bp = 2100' "$SPEC" > "$OUT/trust-regression-negative.json"
go run ./cmd/patchline feedback trust-regression \
  --spec "$OUT/trust-regression-negative.json" \
  --out "$OUT/negative" \
  --json > "$OUT/negative.stdout.json"
jq -e '.ok == false and .summary.failed_checks >= 2' "$OUT/negative/trust-regression.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|source_code|raw_evidence|finding_id|evidence_hash' \
  "$OUT/trust-regression.json" "$OUT/trust-regression.md"; then
  echo "FAIL: trust regression output retained source or raw identifiers" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/trust-regression.json" '{
  version: "patchline.human-trust-regression-gate-results/v1",
  checks: $r[0].summary.checks,
  regression_negative_control: true,
  verified: true
}' > "$OUT/gate-summary.json"

echo "human-trust-regression gate passed: explanation faithfulness and over-reliance regressions are caught"
