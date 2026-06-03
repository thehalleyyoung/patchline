#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/incident-response-drill.json}"
OUT="${2:-results/generated/incident-response-drill-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.incident-response-drill/v1" and
  .criteria.max_detection_minutes == 60 and
  .criteria.max_public_disclosure_hours == 6 and
  .criteria.max_remediation_hours == 48 and
  .criteria.require_public_disclosure == true and
  .criteria.require_regression_gate == true and
  (.drill.timeline | length) == 7 and
  (.drill.roles | length) == 4
' "$SPEC" > /dev/null

for path in $(jq -r '
  (
    .drill.evidence_paths[]?,
    .drill.timeline[].evidence_path?,
    .drill.disclosures[].evidence_path?,
    .drill.remediations[].evidence_path?,
    .drill.remediations[].gate_report_path?,
    .drill.roles[].evidence_path?
  )
  | select(type == "string" and length > 0)
' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Incident-response drill" "incident-response-drill" "make incident-response-drill-gate"; do
  grep -F "$phrase" docs/incident-response-drill.md README.md > /dev/null
done

go test ./internal/incidentdrill -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestIncidentResponseDrillCommandWritesReports

go run ./cmd/patchline incident-response-drill \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/incident-response-drill.json"
test -s "$OUT/safe/incident-response-drill.md"

jq -e '
  .version == "patchline.incident-response-drill-report/v1" and
  .ok == true and
  .summary.timeline_events == 7 and
  .summary.disclosures == 1 and
  .summary.remediations == 2 and
  .summary.regression_gates == 1 and
  .summary.distinct_roles == 4 and
  .summary.detection_minutes == 30 and
  .summary.public_disclosure_hours == 2 and
  .summary.remediation_hours == 12 and
  .summary.counterexamples == 0 and
  ([.drill.remediations[] | select(.kind == "regression_gate" and .hash_matches == true)] | length) == 1
' "$OUT/safe/incident-response-drill.json" > /dev/null

jq '
  (.drill.disclosures[] | select(.id == "status-page-update") | .published_at) = "2026-02-03T03:00:00Z" |
  (.drill.timeline[] | select(.phase == "public_disclosure") | .at) = "2026-02-03T03:00:00Z"
' "$SPEC" > "$OUT/bad-disclosure.json"
go run ./cmd/patchline incident-response-drill \
  --spec "$OUT/bad-disclosure.json" \
  --root . \
  --out "$OUT/bad-disclosure" \
  --json > "$OUT/bad-disclosure.stdout.json"
jq -e '.ok == false and any(.counterexamples[]; .kind == "public_disclosure_deadline_exceeded")' \
  "$OUT/bad-disclosure/incident-response-drill.json" > /dev/null

jq '
  (.drill.remediations[] | select(.id == "detector-regression") | .completed_at) = "2026-02-05T12:00:00Z"
' "$SPEC" > "$OUT/bad-remediation.json"
go run ./cmd/patchline incident-response-drill \
  --spec "$OUT/bad-remediation.json" \
  --root . \
  --out "$OUT/bad-remediation" \
  --json > "$OUT/bad-remediation.stdout.json"
jq -e '.ok == false and any(.counterexamples[]; .kind == "remediation_deadline_exceeded") and any(.counterexamples[]; .kind == "remediation_completed_after_due")' \
  "$OUT/bad-remediation/incident-response-drill.json" > /dev/null

jq '
  (.drill.remediations[] | select(.id == "detector-regression") | .gate_report_sha256) = "0000000000000000000000000000000000000000000000000000000000000000"
' "$SPEC" > "$OUT/bad-hash.json"
go run ./cmd/patchline incident-response-drill \
  --spec "$OUT/bad-hash.json" \
  --root . \
  --out "$OUT/bad-hash" \
  --json > "$OUT/bad-hash.stdout.json"
jq -e '.ok == false and any(.counterexamples[]; .kind == "remediation_gate_hash_mismatch")' \
  "$OUT/bad-hash/incident-response-drill.json" > /dev/null

jq '
  (.drill.disclosures[] | select(.id == "status-page-update") | .summary) = "Public update accidentally included Bearer not-public."
' "$SPEC" > "$OUT/bad-private-marker.json"
go run ./cmd/patchline incident-response-drill \
  --spec "$OUT/bad-private-marker.json" \
  --root . \
  --out "$OUT/bad-private-marker" \
  --json > "$OUT/bad-private-marker.stdout.json"
jq -e '.ok == false and any(.counterexamples[]; .kind == "public_summary_private_marker")' \
  "$OUT/bad-private-marker/incident-response-drill.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/incident-response-drill.json")"
go run ./cmd/patchline incident-response-drill \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/incident-response-drill.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: incident-response drill report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/incident-response-drill.json" '{
  version: "patchline.incident-response-drill-gate-results/v1",
  timeline_events: $safe[0].summary.timeline_events,
  disclosures: $safe[0].summary.disclosures,
  remediations: $safe[0].summary.remediations,
  regression_gates: $safe[0].summary.regression_gates,
  detection_minutes: $safe[0].summary.detection_minutes,
  public_disclosure_hours: $safe[0].summary.public_disclosure_hours,
  remediation_hours: $safe[0].summary.remediation_hours,
  delayed_disclosure_rejected: true,
  delayed_remediation_rejected: true,
  hash_mismatch_rejected: true,
  private_marker_rejected: true,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "incident-response-drill gate passed: false-negative drill validates disclosure/remediation timelines, evidence hashes, regression gate closure, and negative controls"
