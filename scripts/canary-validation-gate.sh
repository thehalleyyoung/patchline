#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/canary-validation-gate.json}"
BEFORE="${2:-examples/canary-before-snapshot.json}"
AFTER_GOOD="${3:-examples/canary-after-snapshot-good.json}"
AFTER_BAD="${4:-examples/canary-after-snapshot-bad.json}"
OUT="${5:-results/generated/canary-validation-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.canary-validation/v1" and
  .sample_policy.redacted == true and
  .sample_policy.production_like == true and
  (.invariants | length) >= 6
' "$SPEC" > /dev/null

for phrase in "Canary-data validation protocol" "canary-validate" "make canary-validation-gate"; do
  grep -F "$phrase" docs/canary-validation.md README.md > /dev/null
done

go test ./internal/canaryvalidate -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestCanaryValidateCommandWritesReports

go run ./cmd/patchline canary-validate \
  --spec "$SPEC" \
  --before "$BEFORE" \
  --after "$AFTER_GOOD" \
  --out "$OUT/good" \
  --json > "$OUT/good.stdout.json"

test -s "$OUT/good/canary-validation.json"
test -s "$OUT/good/canary-validation.md"
test -s "$OUT/good/canary-validation.sql"

jq -e '
  .version == "patchline.canary-validation-report/v1" and
  .ok == true and
  .summary.rows_before == 3 and
  .summary.rows_after == 3 and
  .summary.matched_rows == 3 and
  .summary.checked == 6 and
  .summary.refuted == 0 and
  .summary.protocol_refuted == 0 and
  .privacy.hash_only_evidence == true and
  .privacy.raw_values_emitted == false and
  .privacy.row_values_emitted == false and
  .privacy.redaction_salt_emitted == false and
  (.protocol | all(.status == "checked")) and
  (.invariants | all(.status == "checked"))
' "$OUT/good/canary-validation.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/good/canary-validation.json")"
go run ./cmd/patchline canary-validate \
  --spec "$SPEC" \
  --before "$BEFORE" \
  --after "$AFTER_GOOD" \
  --out "$OUT/good-repeat" \
  --json > "$OUT/good-repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/good-repeat/canary-validation.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: canary validation report hash is not deterministic" >&2
  exit 1
fi

go run ./cmd/patchline canary-validate \
  --spec "$SPEC" \
  --before "$BEFORE" \
  --after "$AFTER_BAD" \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  .summary.checked == 1 and
  .summary.refuted == 5 and
  .summary.violations >= 6 and
  (.invariants[] | select(.id == "invoice-row-count") | .status == "checked") and
  (.invariants[] | select(.id == "external-id-not-null") | .status == "refuted") and
  (.invariants[] | select(.id == "external-id-unique") | .violations | any(.code == "duplicate_value")) and
  (.invariants[] | select(.id == "external-id-derived") | .violations | any(.code == "target_missing" or .code == "stale_target")) and
  (.invariants[] | select(.id == "stable-business-fields") | .violations | any(.code == "unexpected_change")) and
  (.invariants[] | select(.id == "only-external-id-changes") | .violations | any(.code == "disallowed_column_change"))
' "$OUT/bad/canary-validation.json" > /dev/null

if grep -Eiq 'inv_redacted_|acct_[abc]_redacted|local-canary-validation-gate-salt-2026|status": "paid|status": "open|status": "disabled' \
  "$OUT/good/canary-validation.json" "$OUT/good/canary-validation.md" "$OUT/good.stdout.json" \
  "$OUT/good-repeat/canary-validation.json" "$OUT/bad/canary-validation.json" "$OUT/bad/canary-validation.md" "$OUT/bad.stdout.json"; then
  echo "FAIL: canary validation output retained raw canary values, sampled row ids, or redaction salt" >&2
  exit 1
fi

jq -n --slurpfile good "$OUT/good/canary-validation.json" --slurpfile bad "$OUT/bad/canary-validation.json" '{
  version: "patchline.canary-validation-gate-results/v1",
  checked_invariants: $good[0].summary.checked,
  matched_rows: $good[0].summary.matched_rows,
  negative_refuted_invariants: $bad[0].summary.refuted,
  deterministic_hash: $good[0].hash,
  hash_only_evidence: $good[0].privacy.hash_only_evidence,
  verified: true
}' > "$OUT/gate-summary.json"

echo "canary-validation gate passed: pre/post redacted canary snapshots checked with hash-only counterexamples"
