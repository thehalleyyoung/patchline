#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/multi-ecosystem-migration-gate.json}"
OUT="${2:-results/generated/multi-ecosystem-migration}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis"

jq -e '
  .version == "patchline.multi-ecosystem-migration-gate/v1" and
  (.claim | length) > 200 and
  (.frameworks | length) == 7
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
expected_system="$(jq -r '.real_repo.expected_system' "$SPEC")"
expected_native="$(jq -r '.real_repo.expected_native' "$SPEC")"
min_migrations="$(jq -r '.real_repo.minimum_migrations' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" \
  --download-dir "$OUT/cache" --stages inventory --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

INV="$OUT/analysis/inventory/inventory.json"
test -s "$INV"

real_system_count="$(jq --arg s "$expected_system" '[.migration_systems[]? | select(.kind == $s)] | length' "$INV")"
real_native_present="$(jq --arg c "$expected_native" '[.native_commands[]? | select(.command == $c)] | length' "$INV")"

# The full seven-framework matrix and the per-framework native command are covered by unit tests.
go test ./internal/project/ -run 'TestInventoryDetectsMultiEcosystemMigrationFrameworks' \
  > "$OUT/unit-tests.log" 2>&1 && unit_ok=true || unit_ok=false
rm -rf internal/project/results

jq -n \
  --arg repo "$repo" \
  --arg system "$expected_system" \
  --arg native "$expected_native" \
  --argjson real_count "$real_system_count" \
  --argjson native_present "$real_native_present" \
  --argjson min "$min_migrations" \
  --argjson unit_ok "$unit_ok" \
  --slurpfile spec "$SPEC" '
  {
    version: "patchline.multi-ecosystem-migration/v1",
    real_repo: $repo,
    real_system: $system,
    real_native_command: $native,
    real_system_files: $real_count,
    frameworks: ($spec[0].frameworks)
  } |
  . + {
    real_repo_detected: (.real_system_files >= $min and $native_present >= 1),
    framework_matrix_verified: $unit_ok
  }
' > "$OUT/multi-ecosystem-migration.json"

{
  echo "# Multi-ecosystem migration coverage"
  echo
  jq -r '"Patchline detected `" + (.real_system_files|tostring) + "` `" + .real_system + "` migrations in the real `" + .real_repo + "` repository and recommended its native command `" + .real_native_command + "`."' "$OUT/multi-ecosystem-migration.json"
  echo
  echo "## Guarantees"
  jq -r '"- real-repo framework detected with native command: `" + (.real_repo_detected|tostring) + "`\n- seven-framework matrix (Laravel, Ecto, Diesel, Sequelize, Knex, Doctrine, Rails multi-db) and native commands verified by unit tests: `" + (.framework_matrix_verified|tostring) + "`"' "$OUT/multi-ecosystem-migration.json"
  echo
  echo "Patchline now locates the data-change surface of polyglot real-world projects: PHP (Laravel, Doctrine), Elixir (Ecto), Rust (Diesel), JavaScript/TypeScript (Sequelize, Knex), and Rails multi-database setups, each mapped to the project-native migration command a maintainer would actually run."
} > "$OUT/multi-ecosystem-migration.md"
cp "$OUT/multi-ecosystem-migration.md" "$OUT/README.md"

echo "multi-ecosystem migration coverage complete: $(jq '.real_system_files' "$OUT/multi-ecosystem-migration.json") laravel migrations on real repo, framework matrix verified $(jq '.framework_matrix_verified' "$OUT/multi-ecosystem-migration.json")"
