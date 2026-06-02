#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/live-feedback-ingestion-gate.json}"
OUT="${2:-results/generated/live-feedback-ingestion-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.live-feedback-ingestion/v1" and (.claim | length) > 200 and (.outcomes | length) >= 10' "$SPEC" > /dev/null

for phrase in "source-free live feedback" "make live-feedback-ingestion-gate"; do
  grep -F "$phrase" docs/live-feedback-ingestion.md README.md > /dev/null
done

go test ./internal/feedback
go run ./cmd/patchline feedback ingest "$SPEC" --out "$OUT" --json > "$OUT.stdout.json"

for output in live-feedback.json live-feedback.md; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.live-feedback/v1" and
  .ok == true and
  .shareable == true and
  .summary.input_records == 11 and
  .summary.accepted_records == 7 and
  .summary.rejected_records == 3 and
  .summary.deduplicated_records == 1 and
  .summary.requested_min_group_size == 1 and
  .summary.effective_min_group_size >= 3 and
  .summary.groups_published == 2 and
  .summary.groups_suppressed == 1 and
  .residual.count == 1 and
  (.groups | all(.count >= 3)) and
  .privacy.source_free == true and
  .privacy.raw_values_free == true and
  .privacy.identifier_free == true and
  .privacy.salt_emitted == false and
  .privacy.unknown_fields_stored == false
' "$OUT/live-feedback.json" > /dev/null

jq -e '[.. | objects | keys[]? | select(. == "finding_id" or . == "evidence_hash" or . == "salt")] | length == 0' "$OUT/live-feedback.json" > /dev/null

if grep -Eiq 'DROP TABLE|UPDATE accounts|diff --git|db/migrate|source_code|raw_evidence|finding_id|evidence_hash|team-alpha|local-secret-feedback-salt-2026' "$OUT/live-feedback.json" "$OUT/live-feedback.md"; then
  echo "FAIL: live feedback output retained source, raw evidence, identifiers, or salt" >&2
  exit 1
fi

jq -n --slurpfile r "$OUT/live-feedback.json" '{
  version: "patchline.live-feedback-ingestion-gate-results/v1",
  accepted_records: $r[0].summary.accepted_records,
  rejected_records: $r[0].summary.rejected_records,
  effective_min_group_size: $r[0].summary.effective_min_group_size,
  verified: true
}' > "$OUT/gate-summary.json"

echo "live-feedback-ingestion gate passed: source-free live feedback aggregated with k-anonymous groups"
