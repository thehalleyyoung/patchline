#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/reviewer-fairness-audit.json}"
OUT="${2:-results/generated/reviewer-fairness-audit-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.reviewer-fairness-audit/v1" and
  .criteria.min_teams == 2 and
  .criteria.min_ecosystems == 2 and
  .criteria.min_reviews_per_group == 2 and
  (.observations | length) == 4
' "$SPEC" > /dev/null

for path in $(jq -r '.observations[].evidence_paths[]' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Reviewer fairness audit" "reviewer-fairness-audit" "make reviewer-fairness-audit-gate"; do
  grep -F "$phrase" docs/reviewer-fairness-audit.md README.md > /dev/null
done

go test ./internal/reviewerfairness -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestReviewerFairnessAuditCommandWritesReports

go run ./cmd/patchline reviewer-fairness-audit \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/reviewer-fairness-audit.json"
test -s "$OUT/safe/reviewer-fairness-audit.md"

jq -e '
  .version == "patchline.reviewer-fairness-audit-report/v1" and
  .ok == true and
  .summary.reviews == 4 and
  .summary.teams == 2 and
  .summary.ecosystems == 2 and
  .summary.team_burden_ratio <= 1.2 and
  .summary.ecosystem_false_positive_rate_gap == 0.25 and
  .summary.team_escalation_rate_gap == 0 and
  .summary.counterexamples == 0 and
  ([.teams[].evidence[] | select(.sha256 | length == 64)] | length) >= 2
' "$OUT/safe/reviewer-fairness-audit.json" > /dev/null

jq '
  (.observations[] | select(.review_id == "payments-django") | .false_positives) = 3
' "$SPEC" > "$OUT/fp-gap-spec.json"

go run ./cmd/patchline reviewer-fairness-audit \
  --spec "$OUT/fp-gap-spec.json" \
  --root . \
  --out "$OUT/fp-gap" \
  --json > "$OUT/fp-gap.stdout.json"

jq -e '
  .ok == false and
  .summary.counterexamples == 1 and
  (.counterexamples | length) == 1 and
  .counterexamples[0].kind == "false_positive_gap" and
  .counterexamples[0].subject == "team"
' "$OUT/fp-gap/reviewer-fairness-audit.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/reviewer-fairness-audit.json")"
go run ./cmd/patchline reviewer-fairness-audit \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/reviewer-fairness-audit.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: reviewer fairness audit hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/reviewer-fairness-audit.json" --slurpfile fp "$OUT/fp-gap/reviewer-fairness-audit.json" '{
  version: "patchline.reviewer-fairness-audit-gate-results/v1",
  reviews: $safe[0].summary.reviews,
  teams: $safe[0].summary.teams,
  ecosystems: $safe[0].summary.ecosystems,
  team_burden_ratio: $safe[0].summary.team_burden_ratio,
  ecosystem_false_positive_rate_gap: $safe[0].summary.ecosystem_false_positive_rate_gap,
  negative_control: $fp[0].counterexamples[0],
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "reviewer fairness audit gate passed: burden, false-positive, and escalation parity are audited across teams and ecosystems"
