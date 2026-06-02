#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/multi-ecosystem-migration-gate.json}"
OUT="${2:-results/generated/multi-ecosystem-migration-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.multi-ecosystem-migration-gate/v1" and (.frameworks|length) == 7' "$SPEC" > /dev/null

for phrase in "Laravel" "Ecto" "Diesel" "Sequelize" "Knex" "Doctrine" "Rails multi-db" "make multi-ecosystem-migration-gate"; do
  grep -F "$phrase" docs/multi-ecosystem-migration.md README.md > /dev/null
done

bash scripts/multi-ecosystem-migration.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in multi-ecosystem-migration.json multi-ecosystem-migration.md README.md; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.multi-ecosystem-migration/v1" and
  .real_system == "laravel" and
  .real_repo_detected == true and
  .framework_matrix_verified == true and
  .real_system_files >= 2 and
  (.frameworks | length) == 7
' "$OUT/multi-ecosystem-migration.json" > /dev/null

jq -n --slurpfile r "$OUT/multi-ecosystem-migration.json" '{
  version: "patchline.multi-ecosystem-migration-gate-results/v1",
  real_system: $r[0].real_system,
  real_system_files: $r[0].real_system_files,
  frameworks: $r[0].frameworks,
  verified: true
}' > "$OUT/gate-summary.json"

echo "multi-ecosystem migration gate passed: laravel detected on real repo, 7-framework matrix verified"
