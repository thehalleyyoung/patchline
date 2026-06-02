#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/certificate-revocation}"
rm -rf "$OUT"
mkdir -p "$OUT"

go run ./tools/certlifecyclefixture --root . --out "$OUT"
go test ./internal/certrevocation -run 'TestReplay' > "$OUT/go-test.log"
go run ./cmd/patchline cert revoke-verify "$OUT/revocation-bundle.json" --json > "$OUT/replay.json"

jq -e '
  .version == "patchline.certificate-revocation-replay/v1"
  and .all_ok == true
  and .records == 1
  and .revoked == 1
  and .checkpoint.count == 1
  and (.checkpoint.tip_hash | test("^[0-9a-f]{64}$"))
' "$OUT/replay.json" > /dev/null

grep -F "signed revocation" docs/certificate-lifecycle.md > /dev/null

echo "certificate-revocation gate passed: signed records replay through a payload-bound hash-chain ledger"
