#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/schema-compat-gate.json}"
OUT="${2:-results/generated/schema-compat}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis"

jq -e '
  .version == "patchline.schema-compat-gate/v1" and
  (.claim | length) > 200
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
min_avro="$(jq -r '.real_repo.minimum_avro_breaking' "$SPEC")"
min_proto="$(jq -r '.real_repo.minimum_proto_breaking' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" \
  --download-dir "$OUT/cache" --stages inventory --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

INV="$OUT/analysis/inventory/inventory.json"
test -s "$INV"

jq '.schema_compatibility // []' "$INV" > "$OUT/schema-compat.json"

avro_breaking="$(jq '[.[] | select(.kind == "avro_field_without_default")] | length' "$OUT/schema-compat.json")"
proto_breaking="$(jq '[.[] | select(.kind == "protobuf_required_field")] | length' "$OUT/schema-compat.json")"

real_repo_detected=false
if [ "$avro_breaking" -ge "$min_avro" ] && [ "$proto_breaking" -ge "$min_proto" ]; then
  real_repo_detected=true
fi

go test ./internal/project/ -run 'TestInventoryDetectsSchemaCompatRisks|TestInventoryDoesNotFlagSchemaCompatForPlainJSON' \
  > "$OUT/unit-tests.log" 2>&1 && unit_ok=true || unit_ok=false
rm -rf internal/project/results

jq -n \
  --arg repo "$repo" \
  --argjson avro "$avro_breaking" \
  --argjson proto "$proto_breaking" \
  --argjson real_detected "$real_repo_detected" \
  --argjson unit_ok "$unit_ok" '
  {
    version: "patchline.schema-compat/v1",
    real_repo: $repo,
    avro_breaking_fields: $avro,
    proto_required_fields: $proto,
    real_repo_detected: $real_detected,
    compat_matrix_verified: $unit_ok
  }
' > "$OUT/schema-compat-summary.json"

{
  echo "# Schema registry and protobuf/Avro compatibility"
  echo
  jq -r '"In the real `" + .real_repo + "` repository Patchline flagged `" + (.avro_breaking_fields|tostring) + "` Avro record fields without defaults and `" + (.proto_required_fields|tostring) + "` proto2 required fields as schema-evolution hazards that put stored data and live consumers at risk."' "$OUT/schema-compat-summary.json"
  echo
  echo "## Guarantees"
  jq -r '"- real-repo Avro and protobuf breaking risks detected: `" + (.real_repo_detected|tostring) + "`\n- compatibility classification and no-false-positive rule verified by unit tests: `" + (.compat_matrix_verified|tostring) + "`"' "$OUT/schema-compat-summary.json"
  echo
  echo "Patchline treats schema evolution as a data-change surface: proto2 required fields, protobuf messages lacking reserved declarations, and Avro fields without defaults are recorded as searchable schema_compatibility facts so breaking changes are reviewed before they corrupt stored records or break consumers."
} > "$OUT/schema-compat.md"
cp "$OUT/schema-compat.md" "$OUT/README.md"

echo "schema-compat gate complete: $(jq '.avro_breaking_fields' "$OUT/schema-compat-summary.json") avro + $(jq '.proto_required_fields' "$OUT/schema-compat-summary.json") proto breaking risks on real repo, matrix verified $(jq '.compat_matrix_verified' "$OUT/schema-compat-summary.json")"
