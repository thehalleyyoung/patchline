#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/certificate-plugfest}"
rm -rf "$OUT"
mkdir -p "$OUT"

go run ./tools/certlifecyclefixture --root . --out "$OUT"
go test ./internal/certplugfest -run 'TestValidate' > "$OUT/go-test.log"
go run ./cmd/patchline cert plugfest --submission "$OUT/plugfest-submission.json" --root . --json > "$OUT/plugfest.json"

jq -e '
  .version == "patchline.certificate-plugfest-report/v1"
  and .all_ok == true
  and .offline == true
  and .conformance_verified == true
  and .normalization_verified == true
  and .diff_verified == true
  and .revocation_verified == true
  and .logs_verified == true
' "$OUT/plugfest.json" > /dev/null

grep -F "plugfest" docs/certificate-lifecycle.md > /dev/null

echo "certificate-plugfest gate passed: offline external-tool submission was recomputed in-process with reproducible logs"
