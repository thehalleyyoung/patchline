#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/schema-compat-gate.json}"
OUT="${2:-results/generated/schema-compat-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.schema-compat-gate/v1"' "$SPEC" > /dev/null

for phrase in "protobuf" "Avro" "required" "default" "reserved" "breaking" "make schema-compat-gate"; do
  grep -F "$phrase" docs/schema-compat.md README.md > /dev/null
done

bash scripts/schema-compat.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in schema-compat-summary.json schema-compat.md README.md schema-compat.json; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.schema-compat/v1" and
  .real_repo_detected == true and
  .compat_matrix_verified == true and
  .avro_breaking_fields >= 5 and
  .proto_required_fields >= 1
' "$OUT/schema-compat-summary.json" > /dev/null

jq -n --slurpfile r "$OUT/schema-compat-summary.json" '{
  version: "patchline.schema-compat-gate-results/v1",
  real_repo: $r[0].real_repo,
  avro_breaking_fields: $r[0].avro_breaking_fields,
  proto_required_fields: $r[0].proto_required_fields,
  verified: true
}' > "$OUT/gate-summary.json"

echo "schema-compat gate passed: Avro and protobuf breaking-change risks detected on real repo, compatibility matrix verified"
