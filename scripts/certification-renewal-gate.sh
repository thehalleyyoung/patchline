#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/certification-renewal.json}"
OUT="${2:-results/generated/certification-renewal-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.certification-renewal/v1" and
  (.claim | length) > 240 and
  (.engine_semantics | length) == 2 and
  (.hazard_classes | length) == 2 and
  (.credentials | map(select(.status == "active")) | length) == 1 and
  (.attempts | length) == 1
' "$SPEC" > /dev/null

for phrase in "Certification renewal" "make certification-renewal-gate"; do
  grep -F "$phrase" docs/certification-renewal.md README.md > /dev/null
done

go test ./internal/certificationrenewal -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestCertificationRenewalCommandWritesReports

go run ./cmd/patchline certification-renewal \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/certification-renewal.json"
test -s "$OUT/safe/certification-renewal.md"

jq -e '
  .version == "patchline.certification-renewal-report/v1" and
  .ok == true and
  .summary.engine_semantics_updates == 2 and
  .summary.new_hazard_classes == 2 and
  .summary.active_credentials == 1 and
  .summary.renewed_credentials == 1 and
  .credentials[0].requires_renewal_from == "2026-03-01" and
  .attempts[0].reviewer_evidence_hash_matches == true and
  ([.engine_semantics[].evidence[] | select(.sha256 | length == 64)] | length) == 4 and
  ([.hazard_classes[].evidence[] | select(.sha256 | length == 64)] | length) == 4 and
  ([.attempts[].evidence[] | select(.sha256 | length == 64)] | length) == 3
' "$OUT/safe/certification-renewal.json" > /dev/null

jq '
  (.attempts[0].submitted_at) = "2026-02-20" |
  (.attempts[0].covered_hazard_classes) = ["replication-lag-risk"] |
  (.attempts[0].covered_topics) = ["postgres-lock-modes", "transactional-ddl", "mysql-online-ddl", "implicit-commit-rollback", "replication-lag-obligations"] |
  (.attempts[0].commands) = [] |
  (.attempts[0].reviewer_evidence_hash) = ""
' "$SPEC" > "$OUT/bad-spec.json"

go run ./cmd/patchline certification-renewal \
  --spec "$OUT/bad-spec.json" \
  --root . \
  --out "$OUT/bad" \
  --json > "$OUT/bad.stdout.json"

jq -e '
  .ok == false and
  (.counterexamples | any(.kind == "stale_renewal_attempt")) and
  (.counterexamples | any(.kind == "missing_hazard_class_coverage")) and
  (.counterexamples | any(.kind == "missing_renewal_topics")) and
  (.counterexamples | any(.kind == "missing_reviewer_evidence_hash")) and
  (.counterexamples | any(.kind == "missing_reproducible_renewal_command")) and
  (.counterexamples | any(.kind == "credential_not_renewed"))
' "$OUT/bad/certification-renewal.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/certification-renewal.json")"
go run ./cmd/patchline certification-renewal \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/certification-renewal.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: certification renewal hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/certification-renewal.json" --slurpfile bad "$OUT/bad/certification-renewal.json" '{
  version: "patchline.certification-renewal-gate-results/v1",
  engine_semantics_updates: $safe[0].summary.engine_semantics_updates,
  new_hazard_classes: $safe[0].summary.new_hazard_classes,
  renewed_credentials: $safe[0].summary.renewed_credentials,
  bad_counterexamples: $bad[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "certification renewal gate passed: engine semantics and hazard-class renewals are gate-backed, hash-bound, and deterministic"
