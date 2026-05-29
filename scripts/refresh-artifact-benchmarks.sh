#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="benchmarks/expected"
mkdir -p "$OUT"

echo "Refreshing Patchline artifact benchmark golden reports"
echo "output_dir=$OUT"

go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/smoke.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/smoke.json --out "$OUT/smoke-report.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/negative.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/negative.json --out "$OUT/negative-report.json"

echo "Fetching pinned public migration corpus for public golden report"
bash scripts/fetch-public-corpus.sh
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_migrations.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_migrations.json --out "$OUT/public-migrations-report.json"

go run ./cmd/patchline artifact-benchmark compare "$OUT/smoke-report.json" "$OUT/smoke-report.json"
go run ./cmd/patchline artifact-benchmark compare "$OUT/negative-report.json" "$OUT/negative-report.json"
go run ./cmd/patchline artifact-benchmark compare "$OUT/public-migrations-report.json" "$OUT/public-migrations-report.json"

echo "golden_reports_refreshed=$OUT"
