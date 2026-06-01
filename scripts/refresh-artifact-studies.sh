#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="benchmarks/expected"
mkdir -p "$OUT"

if [[ "${PATCHLINE_ACCEPT_EXPECTED_REFRESH:-0}" != "1" ]]; then
  echo "Refusing to rewrite study expected manifests without PATCHLINE_ACCEPT_EXPECTED_REFRESH=1" >&2
  echo "Reviewer validation uses make artifact-studies-compare and make artifact-studies-public-compare; refresh is maintainer-only after reviewing an intentional semantic change." >&2
  exit 1
fi

echo "Refreshing Patchline artifact study expected hash manifests"
echo "output_dir=$OUT"

make artifact-studies
go run ./cmd/patchline artifact-study summarize results/generated/artifact-studies --out "$OUT/studies-strict.json"

echo "Fetching pinned public migration corpus for public study expected hash manifest"
make artifact-studies-public
go run ./cmd/patchline artifact-study summarize results/generated/artifact-studies/public-migrations --out "$OUT/studies-public-migrations.json"

echo "study_expected_manifests_refreshed=$OUT"
