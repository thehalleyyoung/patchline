#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/certificate-semantic-diff}"
rm -rf "$OUT"
mkdir -p "$OUT"

go run ./tools/certlifecyclefixture --root . --out "$OUT"
go test ./internal/certdiff -run 'TestCompare' > "$OUT/go-test.log"
go run ./cmd/patchline cert diff \
  specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci \
  "$OUT/weakened-proof-frame.plci" \
  --root . \
  --json > "$OUT/diff.json"

jq -e '
  .version == "patchline.certificate-semantic-diff/v1"
  and .summary.weakened == 1
  and .summary.unchanged == 1
  and (.obligation_lattice | contains("refuted is a counterexample"))
' "$OUT/diff.json" > /dev/null

grep -F "Certificate lifecycle" docs/certificate-lifecycle.md > /dev/null

echo "certificate-semantic-diff gate passed: obligation confidence weakening is classified without collapsing refuted counterexamples"
