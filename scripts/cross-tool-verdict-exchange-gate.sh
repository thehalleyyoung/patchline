#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/cross-tool-verdict-exchange-gate.json}"
OUT="${2:-results/generated/cross-tool-verdict-exchange}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '
  .version == "patchline.cross-tool-verdict-exchange-gate/v1"
  and (.claim | length) > 300
  and (.analyzers | length) == 3
  and .positive_cases == 3
  and .negative_controls >= 3
' "$SPEC" > /dev/null

SPEC_DIR="$(jq -r .spec_dir "$SPEC")"
test -d "$SPEC_DIR"
for phrase in "cross-tool proof-carrying verdict exchange" "make cross-tool-verdict-exchange-gate"; do
  grep -F "$phrase" docs/verdict-exchange.md README.md > /dev/null
done

go test ./internal/verdictx > "$OUT/go-test.log"
go run ./tools/verdictx --spec-dir "$SPEC_DIR" --root . --json > "$OUT/report.json"

jq -e '
  .verified == true
  and (.analyzers | length) == 3
  and .positive_cases == 3
  and .roundtrips == .positive_cases
  and .negative_controls_passed >= 3
  and all(.cases[]; .certificate_accepted == true and .equivalent == true and .original_projection_sha256 == .roundtrip_projection_sha256)
  and all(.negative_controls[]; .passed == true)
' "$OUT/report.json" > /dev/null

jq -n \
  --arg spec "$SPEC" \
  --slurpfile report "$OUT/report.json" '
  {
    version: "patchline.cross-tool-verdict-exchange-gate-results/v1",
    spec: $spec,
    analyzers: $report[0].analyzers,
    positive_cases: $report[0].positive_cases,
    roundtrips: $report[0].roundtrips,
    negative_controls_passed: $report[0].negative_controls_passed,
    verified: $report[0].verified
  }
' > "$OUT/gate-summary.json"

echo "cross-tool-verdict-exchange gate passed: PLCI/1 verdict projections round-trip across strong_migrations, Django, and Prisma"
