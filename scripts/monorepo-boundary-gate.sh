#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/monorepo-boundary-gate.json}"
OUT="${2:-results/generated/monorepo-boundary-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.monorepo-boundary-gate/v1" and (.ecosystems|length) >= 7' "$SPEC" > /dev/null

for phrase in "package-boundary" "Bazel" "Turborepo" "Go workspaces" "no-false-positive" "make monorepo-boundary-gate"; do
  grep -F "$phrase" docs/monorepo-boundary.md README.md > /dev/null
done

bash scripts/monorepo-boundary.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in monorepo-boundary.json monorepo-boundary.md README.md boundaries.json; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.monorepo-boundary/v1" and
  .system == "turborepo" and
  .real_repo_detected == true and
  .expected_paths_present == true and
  .ecosystem_matrix_verified == true and
  .boundary_count >= 4 and
  .nonroot_packages >= 1 and
  (.ecosystems_unit_tested | length) == 7
' "$OUT/monorepo-boundary.json" > /dev/null

# Independently confirm at least one real non-root package boundary exists.
nonroot="$(jq '[.[] | select(.path != ".")] | length' "$OUT/boundaries.json")"
if [ "$nonroot" -lt 1 ]; then echo "no non-root package boundary found"; exit 1; fi

jq -n --slurpfile r "$OUT/monorepo-boundary.json" '{
  version: "patchline.monorepo-boundary-gate-results/v1",
  system: $r[0].system,
  boundary_count: $r[0].boundary_count,
  ecosystems: $r[0].ecosystems_unit_tested,
  verified: true
}' > "$OUT/gate-summary.json"

echo "monorepo boundary gate passed: $(jq '.boundary_count' "$OUT/gate-summary.json") boundaries on real repo, 7-ecosystem matrix verified"
