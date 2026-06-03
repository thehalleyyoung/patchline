#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SAFE_SPEC="${1:-examples/multi-service-rollback-plan.json}"
UNSAFE_SPEC="${2:-examples/multi-service-rollback-plan-unsafe.json}"
OUT="${3:-results/generated/verified-rollback-planner-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.multi-service-rollback-plan/v1" and
  .dependency_bound.max_depth == 4 and
  .data_loss_bound.max_rows == 0 and
  (.services | length) == 3 and
  (.migrations | length) == 4
' "$SAFE_SPEC" > /dev/null

for phrase in "Verified multi-service rollback planner" "multi-service-rollback-plan" "make verified-rollback-planner-gate"; do
  grep -F "$phrase" docs/verified-rollback-planner.md README.md > /dev/null
done

go test ./internal/rollbackplanner -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestMultiServiceRollbackPlanCommandWritesReports

go run ./cmd/patchline multi-service-rollback-plan \
  --spec "$SAFE_SPEC" \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/multi-service-rollback-plan.json"
test -s "$OUT/safe/multi-service-rollback-plan.md"

jq -e '
  .version == "patchline.multi-service-rollback-plan-report/v1" and
  .ok == true and
  .dependency_proof.status == "checked" and
  .data_loss_proof.status == "checked" and
  .summary.rollback_waves == 4 and
  .summary.dependency_depth == 4 and
  .summary.dependency_fanout == 1 and
  .summary.data_loss_rows == 0 and
  (.dependency_proof.rollback_order == ["billing-contract","api-read-shift","ledger-dual-write","billing-expand"]) and
  (.dependency_proof.cross_service_edges | length) == 3
' "$OUT/safe/multi-service-rollback-plan.json" > /dev/null

go run ./cmd/patchline multi-service-rollback-plan \
  --spec "$UNSAFE_SPEC" \
  --out "$OUT/unsafe" \
  --json > "$OUT/unsafe.stdout.json"

jq -e '
  .ok == false and
  .data_loss_proof.status == "refuted" and
  .summary.data_loss_rows == 125 and
  .summary.critical_data_loss_rows == 12 and
  (.counterexamples | any(.id == "data_loss.rollback_unverified.api-read-shift")) and
  (.counterexamples | any(.id == "data_loss.bound.rows")) and
  (.counterexamples | any(.id == "data_loss.bound.critical_rows")) and
  (.counterexamples | any(.id == "data_loss.bound.affected_services"))
' "$OUT/unsafe/multi-service-rollback-plan.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/multi-service-rollback-plan.json")"
go run ./cmd/patchline multi-service-rollback-plan \
  --spec "$SAFE_SPEC" \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/multi-service-rollback-plan.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: multi-service rollback planner hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/multi-service-rollback-plan.json" --slurpfile unsafe "$OUT/unsafe/multi-service-rollback-plan.json" '{
  version: "patchline.verified-rollback-planner-gate-results/v1",
  rollback_order: $safe[0].dependency_proof.rollback_order,
  dependency_depth: $safe[0].summary.dependency_depth,
  dependency_fanout: $safe[0].summary.dependency_fanout,
  unsafe_data_loss_rows: $unsafe[0].summary.data_loss_rows,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "verified rollback planner gate passed: reverse dependency order with explicit data-loss bounds"
