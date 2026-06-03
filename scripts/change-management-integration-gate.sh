#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/change-management-integration.json}"
OUT="${2:-results/generated/change-management-integration-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.change-management/v1" and
  .criteria.min_approval_steps == 2 and
  .criteria.require_distinct_approvers == true and
  .criteria.require_patchline_gate_binding == true and
  .criteria.require_evidence_hashes == true and
  (.workflows | length) == 2
' "$SPEC" > /dev/null

for path in $(jq -r '
  .workflows[]
  | (.evidence_paths[]?, .deployment_controls.rollback_plan_path?, .gates[].report_path?, .approvals[].evidence_path?)
' "$SPEC" | sort -u); do
  test -s "$path"
done

for phrase in "Change-management integration" "change-management-verify" "make change-management-integration-gate"; do
  grep -F "$phrase" docs/change-management-integration.md README.md > /dev/null
done

go test ./internal/changemanagement -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestChangeManagementVerifyCommandWritesReports

go run ./cmd/patchline change-management-verify \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/change-management.json"
test -s "$OUT/safe/change-management.md"

jq -e '
  .version == "patchline.change-management-report/v1" and
  .ok == true and
  .summary.workflows == 2 and
  .summary.blocking_gates == 2 and
  .summary.passed_blocking_gates == 2 and
  .summary.approved_steps == 4 and
  .summary.distinct_approvers == 4 and
  .summary.emergency_workflows == 1 and
  .summary.counterexamples == 0 and
  ([.workflows[].gates[] | select(.hash_matches == true)] | length) == 2
' "$OUT/safe/change-management.json" > /dev/null

jq '
  (.workflows[] | select(.workflow_id == "chg-2026-001-expand-contract") | .approvals[0].gate_ids) = ["missing-gate"]
' "$SPEC" > "$OUT/bypass-spec.json"

go run ./cmd/patchline change-management-verify \
  --spec "$OUT/bypass-spec.json" \
  --root . \
  --out "$OUT/bypass" \
  --json > "$OUT/bypass.stdout.json"

jq -e '
  .ok == false and
  .summary.counterexamples >= 1 and
  ([.counterexamples[] | select(.kind == "approval_references_unknown_gate")] | length) == 1 and
  ([.counterexamples[] | select(.kind == "approval_without_passed_blocking_gate")] | length) == 1
' "$OUT/bypass/change-management.json" > /dev/null

jq '
  (.workflows[] | select(.workflow_id == "chg-2026-002-emergency-guard") | .gates[0].report_sha256) = "0000000000000000000000000000000000000000000000000000000000000000"
' "$SPEC" > "$OUT/hash-mismatch-spec.json"

go run ./cmd/patchline change-management-verify \
  --spec "$OUT/hash-mismatch-spec.json" \
  --root . \
  --out "$OUT/hash-mismatch" \
  --json > "$OUT/hash-mismatch.stdout.json"

jq -e '
  .ok == false and
  ([.counterexamples[] | select(.kind == "gate_report_hash_mismatch")] | length) == 1
' "$OUT/hash-mismatch/change-management.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/change-management.json")"
go run ./cmd/patchline change-management-verify \
  --spec "$SPEC" \
  --root . \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/change-management.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: change-management report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/change-management.json" --slurpfile bypass "$OUT/bypass/change-management.json" --slurpfile mismatch "$OUT/hash-mismatch/change-management.json" '{
  version: "patchline.change-management-gate-results/v1",
  workflows: $safe[0].summary.workflows,
  blocking_gates: $safe[0].summary.blocking_gates,
  approved_steps: $safe[0].summary.approved_steps,
  bypass_negative_control: [$bypass[0].counterexamples[].kind],
  hash_negative_control: [$mismatch[0].counterexamples[].kind],
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "change-management integration gate passed: approval workflows are bound to passed Patchline safety gates with hash evidence"
