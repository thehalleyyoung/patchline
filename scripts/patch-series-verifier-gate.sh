#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SAFE_SPEC="${1:-examples/patch-series-verifier.json}"
UNSAFE_SPEC="${2:-examples/patch-series-verifier-unsafe.json}"
OUT="${3:-results/generated/patch-series-verifier-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.patch-series/v1" and
  .dialect == "postgres" and
  (.invariants | length) == 4 and
  (.pull_requests | length) == 3 and
  (.pull_requests[1].depends_on == ["billing-expand"])
' "$SAFE_SPEC" > /dev/null

for phrase in "Patch-series verifier" "patch-series-verify" "make patch-series-verifier-gate"; do
  grep -F "$phrase" docs/patch-series-verifier.md README.md > /dev/null
done

go test ./internal/patchseries -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestPatchSeriesVerifyCommandWritesReports

go run ./cmd/patchline patch-series-verify \
  --spec "$SAFE_SPEC" \
  --out "$OUT/safe" \
  --json > "$OUT/safe.stdout.json"

test -s "$OUT/safe/patch-series.json"
test -s "$OUT/safe/patch-series.md"

jq -e '
  .version == "patchline.patch-series-report/v1" and
  .ok == true and
  .sequence_proof.status == "checked" and
  .summary.pull_requests == 3 and
  .summary.migrations == 3 and
  .summary.statements == 4 and
  .summary.intermediate_states == 5 and
  .summary.refuted_invariants == 0 and
  (.sequence_proof.order == ["billing-expand","ledger-shadow","api-read-shift"]) and
  ([.pull_requests[].migrations[].statements[] | select(.schema_changed == true)] | length) == 4
' "$OUT/safe/patch-series.json" > /dev/null

go run ./cmd/patchline patch-series-verify \
  --spec "$UNSAFE_SPEC" \
  --out "$OUT/unsafe" \
  --json > "$OUT/unsafe.stdout.json"

jq -e '
  .ok == false and
  .summary.refuted_invariants == 1 and
  (.counterexamples | any(.id | contains("invoice-total-preserved"))) and
  ([.pull_requests[].migrations[].statements[] | select(.sql | contains("DROP COLUMN total_cents"))][0].schema_changed == true) and
  ([.pull_requests[].migrations[].statements[] | select(.status == "refuted")] | length) == 1
' "$OUT/unsafe/patch-series.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/safe/patch-series.json")"
go run ./cmd/patchline patch-series-verify \
  --spec "$SAFE_SPEC" \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/patch-series.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: patch-series verifier hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile safe "$OUT/safe/patch-series.json" --slurpfile unsafe "$OUT/unsafe/patch-series.json" '{
  version: "patchline.patch-series-verifier-gate-results/v1",
  order: $safe[0].sequence_proof.order,
  intermediate_states: $safe[0].summary.intermediate_states,
  unsafe_counterexamples: $unsafe[0].counterexamples,
  deterministic_hash: $safe[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "patch-series verifier gate passed: every statement boundary preserves invariants; unsafe drop is refuted"
