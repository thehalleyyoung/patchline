#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="benchmarks/expected"
mkdir -p "$OUT"

if [[ "${PATCHLINE_ACCEPT_EXPECTED_REFRESH:-0}" != "1" ]]; then
  echo "Refusing to rewrite benchmark expected outputs without PATCHLINE_ACCEPT_EXPECTED_REFRESH=1" >&2
  echo "Reviewer validation uses make artifact-benchmark-compare; refresh is maintainer-only after reviewing an intentional semantic change." >&2
  exit 1
fi

echo "Refreshing Patchline artifact benchmark golden reports"
echo "output_dir=$OUT"

go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/smoke.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/smoke.json --out "$OUT/smoke-report.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/negative.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/negative.json --out "$OUT/negative-report.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/repair_cases.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/repair_cases.json --out "$OUT/repair-cases-report.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/semantic_regressions.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/semantic_regressions.json --out "$OUT/semantic-regressions-report.json"

echo "Fetching pinned public migration corpus for public golden report"
bash scripts/fetch-public-corpus.sh
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_migrations.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_migrations.json --out "$OUT/public-migrations-report.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_incidents.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_incidents.json --out "$OUT/public-incidents-report.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_repairs.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_repairs.json --out "$OUT/public-repairs-report.json"
go run ./cmd/patchline artifact-benchmark validate benchmarks/manifests/public_archive.json
go run ./cmd/patchline artifact-benchmark run benchmarks/manifests/public_archive.json --out "$OUT/public-archive-report.json"

echo "golden_reports_refreshed=$OUT"
