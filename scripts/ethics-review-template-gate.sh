#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/ethics-review-template.json}"
OUT="${2:-results/generated/ethics-review-template-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.ethics-review-template/v1" and
  .as_of_date == "2026-03-01T00:00:00Z" and
  (.criteria.required_review_areas | sort) == (["adopter_outcome_study","live_feedback_loop","new_data_source"] | sort) and
  .criteria.min_independent_reviewers == 2 and
  .criteria.max_risk_score == 0.7 and
  .criteria.review_cadence_days == 120 and
  .criteria.require_consent_basis == true and
  .criteria.require_privacy_review == true and
  .criteria.require_retention_policy == true and
  .criteria.require_minimization == true and
  .criteria.require_withdrawal_path == true and
  .criteria.require_security_owner == true and
  .criteria.require_evidence_paths == true and
  .criteria.require_human_oversight_for_feedback == true and
  .criteria.require_preregistration_for_outcome_studies == true and
  .criteria.min_mitigations_per_high_risk_entry == 2 and
  (.entries | length) == 3
' "$SPEC" > /dev/null

for path in $(jq -r '.entries[].evidence_paths[]?' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Ethics review template" "ethics-review-template" "make ethics-review-template-gate"; do
  grep -F "$phrase" docs/ethics-review-template.md README.md > /dev/null
done

go test ./internal/ethicsreview -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestEthicsReviewTemplateCommandWritesReports

go run ./cmd/patchline ethics-review-template \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/ethics-review-template.json"
test -s "$OUT/safe/ethics-review-template.md"

jq -e '
  .version == "patchline.ethics-review-template-report/v1" and
  .ok == true and
  .summary.areas == 3 and
  .summary.entries == 3 and
  .summary.evidence_files >= 5 and
  .summary.high_risk_entries == 0 and
  .summary.min_independent_reviewers == 2 and
  .summary.counterexamples == 0 and
  ([.areas[] | select(.area == "live_feedback_loop" and (.evidence | length >= 2))] | length) == 1 and
  ([.areas[].evidence[] | select(.sha256 | length == 64)] | length) >= 5
' "$OUT/safe/ethics-review-template.json" > /dev/null

jq '
  (.entries[] | select(.review_id == "live-feedback-calibration") | .human_oversight) = ""
' "$SPEC" > "$OUT/missing-human-oversight.json"
go run ./cmd/patchline ethics-review-template \
  --spec "$OUT/missing-human-oversight.json" \
  --root . \
  --out "$OUT/missing-human-oversight" \
  --json > "$OUT/missing-human-oversight.stdout.json"
jq -e '.ok == false and any(.counterexamples[]; .kind == "missing_human_oversight" and .subject == "live-feedback-calibration")' \
  "$OUT/missing-human-oversight/ethics-review-template.json" > /dev/null

jq '
  (.entries[] | select(.review_id == "adopter-review-time-study") | .preregistration) = ""
' "$SPEC" > "$OUT/missing-preregistration.json"
go run ./cmd/patchline ethics-review-template \
  --spec "$OUT/missing-preregistration.json" \
  --root . \
  --out "$OUT/missing-preregistration" \
  --json > "$OUT/missing-preregistration.stdout.json"
jq -e '.ok == false and any(.counterexamples[]; .kind == "missing_preregistration" and .subject == "adopter-review-time-study")' \
  "$OUT/missing-preregistration/ethics-review-template.json" > /dev/null

jq '
  (.entries[] | select(.review_id == "data-source-public-incidents") | .risk_score) = 0.95 |
  (.entries[] | select(.review_id == "data-source-public-incidents") | .last_reviewed) = "2025-01-01T00:00:00Z" |
  (.entries[] | select(.review_id == "data-source-public-incidents") | .mitigations) = [] |
  (.entries[] | select(.review_id == "data-source-public-incidents") | .reviewer_roles) = ["maintainer"] |
  (.entries[] | select(.review_id == "data-source-public-incidents") | .evidence_paths) = ["../outside.md"]
' "$SPEC" > "$OUT/risky-stale-escaped.json"
go run ./cmd/patchline ethics-review-template \
  --spec "$OUT/risky-stale-escaped.json" \
  --root . \
  --out "$OUT/risky-stale-escaped" \
  --json > "$OUT/risky-stale-escaped.stdout.json"
jq -e '
  .ok == false and
  any(.counterexamples[]; .kind == "risk_score_exceeded" and .subject == "data-source-public-incidents") and
  any(.counterexamples[]; .kind == "stale_review" and .subject == "data-source-public-incidents") and
  any(.counterexamples[]; .kind == "invalid_evidence_path" and .subject == "data-source-public-incidents") and
  any(.counterexamples[]; .kind == "missing_entry_evidence" and .subject == "data-source-public-incidents") and
  any(.counterexamples[]; .kind == "insufficient_high_risk_mitigations" and .subject == "data-source-public-incidents") and
  any(.counterexamples[]; .kind == "insufficient_independent_reviewers" and .subject == "data-source-public-incidents")
' "$OUT/risky-stale-escaped/ethics-review-template.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/ethics-review-template.json")"
go run ./cmd/patchline ethics-review-template \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/ethics-review-template.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: ethics-review-template report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/ethics-review-template.json" --slurpfile risky "$OUT/risky-stale-escaped/ethics-review-template.json" '{
  version: "patchline.ethics-review-template-gate-results/v1",
  areas: $safe[0].summary.areas,
  reviews: $safe[0].summary.entries,
  evidence_files: $safe[0].summary.evidence_files,
  deterministic_hash: $safe[0].hash,
  risky_negative_control: [$risky[0].counterexamples[].kind],
  verified: true
}' > "$OUT/gate-summary.json"

echo "ethics-review-template gate passed: data-source, live-feedback, and outcome-study reviews are checked with hashed evidence, oversight/preregistration obligations, and negative controls"
