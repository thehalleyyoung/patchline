#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/explainability-audit.json}"
OUT="${2:-results/generated/explainability-audit-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.explainability-audit/v1" and
  .criteria.min_reviewers == 2 and
  .criteria.min_verdicts == 2 and
  .criteria.min_reviews_per_verdict == 2 and
  .criteria.min_agreement_rate == 1 and
  .criteria.min_supported_rate == 1 and
  .criteria.max_unsupported_rate == 0 and
  .criteria.require_independent_reviewers == true and
  (.reviews | length) == 4
' "$SPEC" > /dev/null

for path in $(jq -r '.reviews[].evidence_paths[]' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Explainability audit" "explainability-audit" "make explainability-audit-gate"; do
  grep -F "$phrase" docs/explainability-audit.md README.md > /dev/null
done

go test ./internal/explainabilityaudit -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestExplainabilityAuditCommandWritesReports

go run ./cmd/patchline explainability-audit \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/explainability-audit.json"
test -s "$OUT/safe/explainability-audit.md"

jq -e '
  .version == "patchline.explainability-audit-report/v1" and
  .ok == true and
  .summary.reviews == 4 and
  .summary.reviewers == 2 and
  .summary.independent_reviewers == 2 and
  .summary.verdicts == 2 and
  .summary.supported_rate == 1 and
  .summary.unsupported_rate == 0 and
  .summary.min_agreement_rate == 1 and
  .summary.counterexamples == 0 and
  ([.verdicts[].evidence[] | select(.sha256 | length == 64)] | length) >= 4
' "$OUT/safe/explainability-audit.json" > /dev/null

jq '
  (.reviews[] | select(.review_id == "claims-evidence-review-b") | .judgment) = "unsupported" |
  (.reviews[] | select(.review_id == "claims-evidence-review-b") | .missing_evidence_notes) = "reviewer could not connect the verdict to the cited evidence trail"
' "$SPEC" > "$OUT/unsupported-review.json"

go run ./cmd/patchline explainability-audit \
  --spec "$OUT/unsupported-review.json" \
  --root . \
  --out "$OUT/unsupported-review" \
  --json > "$OUT/unsupported-review.stdout.json"

jq -e '
  .ok == false and
  any(.counterexamples[]; .kind == "verdict_disagreement" and .subject == "verdict-claims-evidence") and
  any(.counterexamples[]; .kind == "low_supported_rate") and
  any(.counterexamples[]; .kind == "high_unsupported_rate")
' "$OUT/unsupported-review/explainability-audit.json" > /dev/null

jq '
  (.reviews[] | select(.review_id == "walkthrough-review-a") | .expected_evidence_hashes) = {
    "docs/reviewer-walkthrough.md": "0000000000000000000000000000000000000000000000000000000000000000"
  }
' "$SPEC" > "$OUT/hash-mismatch.json"

go run ./cmd/patchline explainability-audit \
  --spec "$OUT/hash-mismatch.json" \
  --root . \
  --out "$OUT/hash-mismatch" \
  --json > "$OUT/hash-mismatch.stdout.json"

jq -e '
  .ok == false and
  any(.counterexamples[]; .kind == "hash_mismatch" and .subject == "walkthrough-review-a")
' "$OUT/hash-mismatch/explainability-audit.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/explainability-audit.json")"
go run ./cmd/patchline explainability-audit \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/explainability-audit.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: explainability-audit report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/explainability-audit.json" --slurpfile unsupported "$OUT/unsupported-review/explainability-audit.json" --slurpfile mismatch "$OUT/hash-mismatch/explainability-audit.json" '{
  version: "patchline.explainability-audit-gate-results/v1",
  reviews: $safe[0].summary.reviews,
  reviewers: $safe[0].summary.reviewers,
  verdicts: $safe[0].summary.verdicts,
  supported_rate: $safe[0].summary.supported_rate,
  unsupported_negative_control: [$unsupported[0].counterexamples[].kind],
  hash_mismatch_negative_control: [$mismatch[0].counterexamples[].kind],
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "explainability-audit gate passed: independent reviewers agree evidence trails support stated verdicts, with disagreement and hash-mismatch negative controls"
