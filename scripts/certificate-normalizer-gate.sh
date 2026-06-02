#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/certificate-normalizer}"
rm -rf "$OUT"
mkdir -p "$OUT"

go test ./internal/certlang -run 'TestNormalize' > "$OUT/go-test.log"
go run ./cmd/patchline cert normalize \
  specs/certificate-interchange/v1/vectors/valid/patchline-proof-frame.plci \
  --root . \
  --out "$OUT/normalized.plci" \
  --json > "$OUT/normalize.json"
go run ./cmd/patchline cert normalize "$OUT/normalized.plci" --root . --json > "$OUT/renormalize.json"

jq -e '
  .version == "PLCI/1"
  and .certificate_id == "patchline.proof.frame.v1"
  and (.normalized_canonical_sha256 | test("^[0-9a-f]{64}$"))
' "$OUT/normalize.json" > /dev/null

jq -e --slurpfile first "$OUT/normalize.json" '
  .normalized_canonical_sha256 == $first[0].normalized_canonical_sha256
  and .changed == false
' "$OUT/renormalize.json" > /dev/null

grep -F "normalize before signing" docs/certificate-lifecycle.md > /dev/null

echo "certificate-normalizer gate passed: equivalent certificate witnesses normalize to a stable signed identity"
