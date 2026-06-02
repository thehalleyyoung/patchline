#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/monorepo-boundary-gate.json}"
OUT="${2:-results/generated/monorepo-boundary}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis"

jq -e '
  .version == "patchline.monorepo-boundary-gate/v1" and
  (.claim | length) > 200 and
  (.ecosystems | length) >= 7
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
expected_system="$(jq -r '.real_repo.expected_system' "$SPEC")"
min_boundaries="$(jq -r '.real_repo.minimum_boundaries' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

INV="$OUT/analysis/inventory/inventory.json"
test -s "$INV"

jq '.package_boundaries' "$INV" > "$OUT/boundaries.json"

boundary_count="$(jq '[.[] | select(.system == "'"$expected_system"'")] | length' "$OUT/boundaries.json")"
nonroot_packages="$(jq '[.[] | select(.system == "'"$expected_system"'" and .path != ".")] | length' "$OUT/boundaries.json")"

# Each expected package path must be present.
missing=0
for p in $(jq -r '.real_repo.expected_paths[]' "$SPEC"); do
  present="$(jq --arg p "$p" '[.[] | select(.path == $p)] | length' "$OUT/boundaries.json")"
  if [ "$present" -lt 1 ]; then echo "missing expected boundary: $p"; missing=$((missing+1)); fi
done

# Independently re-run the boundary detection unit tests that cover the full ecosystem matrix and
# the no-false-positive rule (a lone BUILD file or package.json without a workspace marker).
go test ./internal/project/ \
  -run 'TestInventoryDetectsMonorepoPackageBoundaries|TestInventoryIgnoresBuildFilesWithoutWorkspace' \
  > "$OUT/unit-tests.log" 2>&1 && unit_ok=true || unit_ok=false
rm -rf internal/project/results

jq -n \
  --arg system "$expected_system" \
  --argjson count "$boundary_count" \
  --argjson nonroot "$nonroot_packages" \
  --argjson min "$min_boundaries" \
  --argjson missing "$missing" \
  --argjson unit_ok "$unit_ok" \
  --slurpfile boundaries "$OUT/boundaries.json" \
  --arg repo "$repo" '
  {
    version: "patchline.monorepo-boundary/v1",
    real_repo: $repo,
    system: $system,
    boundary_count: $count,
    nonroot_packages: $nonroot,
    ecosystems_unit_tested: ["bazel","pants","nx","turborepo","maven","gradle","go-workspace"],
    boundaries: ($boundaries[0] // [])
  } |
  . + {
    real_repo_detected: (.boundary_count >= $min and .nonroot_packages >= 1),
    expected_paths_present: ($missing == 0),
    ecosystem_matrix_verified: $unit_ok
  }
' > "$OUT/monorepo-boundary.json"

{
  echo "# Monorepo package-boundary detection"
  echo
  jq -r '"Patchline detected `" + (.boundary_count|tostring) + "` `" + .system + "` package boundaries (" + (.nonroot_packages|tostring) + " packages plus the workspace root) in the real `" + .real_repo + "` monorepo."' "$OUT/monorepo-boundary.json"
  echo
  echo "## Guarantees"
  jq -r '"- real-repo boundaries detected: `" + (.real_repo_detected|tostring) + "`\n- expected package paths present: `" + (.expected_paths_present|tostring) + "`\n- full ecosystem matrix (bazel, pants, nx, turborepo, maven, gradle, go workspaces) and no-false-positive rule verified by unit tests: `" + (.ecosystem_matrix_verified|tostring) + "`"' "$OUT/monorepo-boundary.json"
  echo
  echo "Package boundaries let Patchline attribute a risky migration or destructive write to the owning package inside a large monorepo, rather than reporting it against an undifferentiated repository root. Per-directory build files only become boundaries when a workspace marker confirms the monorepo, so incidental files named BUILD or package.json do not create spurious packages."
} > "$OUT/monorepo-boundary.md"
cp "$OUT/monorepo-boundary.md" "$OUT/README.md"

echo "monorepo boundary detection complete: $(jq '.boundary_count' "$OUT/monorepo-boundary.json") $expected_system boundaries on real repo, ecosystem matrix verified $(jq '.ecosystem_matrix_verified' "$OUT/monorepo-boundary.json")"
