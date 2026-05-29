#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="benchmarks/expected"
mkdir -p "$OUT"

echo "Refreshing Patchline artifact study expected hash manifests"
echo "output_dir=$OUT"

make artifact-studies
go run ./cmd/patchline artifact-study summarize results/generated/artifact-studies --out "$OUT/studies-strict.json"

echo "Fetching pinned public migration corpus for public study expected hash manifest"
make artifact-studies-public
go run ./cmd/patchline artifact-study summarize results/generated/artifact-studies/public-migrations --out "$OUT/studies-public-migrations.json"

go run ./cmd/patchline artifact-study compare results/generated/artifact-studies "$OUT/studies-strict.json"
go run ./cmd/patchline artifact-study compare results/generated/artifact-studies/public-migrations "$OUT/studies-public-migrations.json"

echo "study_expected_manifests_refreshed=$OUT"
