#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/repair-risk-escrow.json}"
OUT="${2:-results/generated/repair-risk-escrow-gate}"

rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.repair-escrow/v1" and
  .thresholds.manual_reviews == 2 and
  .thresholds.certificates == 2 and
  .thresholds.evidence == 2 and
  (.repairs | length) >= 5
' "$SPEC" > /dev/null

for phrase in "Repair-risk escrow" "repair-escrow" "make repair-risk-escrow-gate"; do
  grep -F "$phrase" docs/repair-risk-escrow.md README.md > /dev/null
done

go test ./internal/repairescrow -run 'TestBuildReport|TestReadSpec'
go test ./cmd/patchline -run TestRepairEscrowCommandWritesReports

go run ./cmd/patchline repair-escrow \
  --spec "$SPEC" \
  --out "$OUT/report" \
  --json > "$OUT/stdout.json"

test -s "$OUT/report/repair-escrow.json"
test -s "$OUT/report/repair-escrow.md"

jq -e '
  .version == "patchline.repair-escrow-report/v1" and
  .ok == false and
  .summary.repairs == 5 and
  .summary.released == 1 and
  .summary.held == 2 and
  .summary.rejected == 2 and
  (.repairs[] | select(.id == "release-external-id") | .status == "released") and
  (.repairs[] | select(.id == "hold-missing-certificate") | .status == "held" and (.obligations | map(.id) == ["certificate.threshold"])) and
  (.repairs[] | select(.id == "hold-missing-review") | .status == "held" and .manual_reviews.accepted == 1 and .manual_reviews.duplicates == 1 and (.obligations | map(.id) == ["manual_review.threshold"])) and
  (.repairs[] | select(.id == "reject-manual-review") | .status == "rejected" and (.counterexamples | any(.kind == "manual_review" and (.reason | contains("rejected"))))) and
  (.repairs[] | select(.id == "reject-revoked-certificate") | .status == "rejected" and (.counterexamples | any(.kind == "certificate" and (.reason | contains("revoked")))))
' "$OUT/report/repair-escrow.json" > /dev/null

first_hash="$(jq -r '.hash' "$OUT/report/repair-escrow.json")"
go run ./cmd/patchline repair-escrow \
  --spec "$SPEC" \
  --out "$OUT/repeat" \
  --json > "$OUT/repeat.stdout.json"
second_hash="$(jq -r '.hash' "$OUT/repeat/repair-escrow.json")"
if [[ "$first_hash" != "$second_hash" ]]; then
  echo "FAIL: repair escrow report hash is not deterministic" >&2
  exit 1
fi

jq -n --slurpfile report "$OUT/report/repair-escrow.json" '{
  version: "patchline.repair-risk-escrow-gate-results/v1",
  released: $report[0].summary.released,
  held: $report[0].summary.held,
  rejected: $report[0].summary.rejected,
  deterministic_hash: $report[0].hash,
  verified: true
}' > "$OUT/gate-summary.json"

echo "repair-risk-escrow gate passed: release requires distinct manual review and certificate thresholds"
