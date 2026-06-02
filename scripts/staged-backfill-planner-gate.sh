#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/staged-backfill-plan.json}"
COMPLETE_STORE="${2:-examples/staged-backfill-store-complete.json}"
INCOMPLETE_STORE="${3:-examples/staged-backfill-store-incomplete.json}"
OUT="${4:-results/generated/staged-backfill-planner-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.backfill-plan/v1" and
  .table == "invoices" and
  .target_column == "external_id" and
  (.stages | map(.id) | index("contract")) and
  (.stages | map(.id) | index("delete-compatibility"))
' "$SPEC" > /dev/null

for phrase in "Staged data-backfill planner" "backfill-plan" "make staged-backfill-planner-gate"; do
  grep -F "$phrase" docs/staged-backfill-planner.md README.md > /dev/null
done

go test ./internal/backfillplanner -run 'TestBuildPlan|TestReadSpec'
go test ./cmd/patchline -run TestBackfillPlanCommandWritesReports

go run ./cmd/patchline backfill-plan \
  --spec "$SPEC" \
  --store "$COMPLETE_STORE" \
  --out "$OUT/complete" \
  --json > "$OUT/complete.stdout.json"

test -s "$OUT/complete/backfill-plan.json"
test -s "$OUT/complete/backfill-plan.md"
test -s "$OUT/complete/backfill-plan.sql"

jq -e '
  .version == "patchline.backfill-plan-report/v1" and
  .ok == true and
  .proof.status == "checked" and
  .summary.rows_checked == 3 and
  .summary.counterexamples == 0 and
  (.stages | all(.ready == true)) and
  (.sql | map(.stage_id) | index("contract"))
' "$OUT/complete/backfill-plan.json" > /dev/null

go run ./cmd/patchline backfill-plan \
  --spec "$SPEC" \
  --store "$INCOMPLETE_STORE" \
  --out "$OUT/incomplete" \
  --json > "$OUT/incomplete.stdout.json"

jq -e '
  .ok == false and
  .proof.status == "refuted" and
  (.proof.counterexamples | map(.row_id + ":" + .code) == ["3:target_empty"]) and
  (.stages[] | select(.id == "contract") | .ready == false) and
  (.stages[] | select(.id == "delete-compatibility") | .ready == false)
' "$OUT/incomplete/backfill-plan.json" > /dev/null

jq -n --slurpfile c "$OUT/complete/backfill-plan.json" --slurpfile i "$OUT/incomplete/backfill-plan.json" '{
  version: "patchline.staged-backfill-planner-gate-results/v1",
  complete_rows_checked: $c[0].summary.rows_checked,
  incomplete_counterexamples: ($i[0].proof.counterexamples | map(.row_id + ":" + .code)),
  contract_blocked_until_complete: (($i[0].stages[] | select(.id == "contract") | .ready) == false),
  verified: true
}' > "$OUT/gate-summary.json"

echo "staged-backfill-planner gate passed: contract and compatibility deletion wait for checked completeness"
