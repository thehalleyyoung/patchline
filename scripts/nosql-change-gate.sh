#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/nosql-change-gate.json}"
OUT="${2:-results/generated/nosql-change-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.nosql-change-gate/v1" and (.engines|length) == 5' "$SPEC" > /dev/null

for phrase in "MongoDB" "Cassandra" "Elasticsearch" "Redis" "DynamoDB" "destructive" "make nosql-change-gate"; do
  grep -F "$phrase" docs/nosql-change.md README.md > /dev/null
done

bash scripts/nosql-change.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in nosql-change.json nosql-change.md README.md nosql-changes.json; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.nosql-change/v1" and
  .engine == "cassandra" and
  .real_repo_destructive_detected == true and
  .engine_matrix_verified == true and
  .destructive_changes >= 3 and
  .engine_changes >= 1 and
  (.engines | length) == 5
' "$OUT/nosql-change.json" > /dev/null

# Independently confirm at least one destructive cassandra drop is present.
drops="$(jq '[.[] | select(.kind == "cassandra:dropTable" or .kind == "cassandra:dropKeyspace")] | length' "$OUT/nosql-changes.json")"
if [ "$drops" -lt 1 ]; then echo "no destructive cassandra drop detected"; exit 1; fi

jq -n --slurpfile r "$OUT/nosql-change.json" '{
  version: "patchline.nosql-change-gate-results/v1",
  engine: $r[0].engine,
  engine_changes: $r[0].engine_changes,
  destructive_changes: $r[0].destructive_changes,
  engines: $r[0].engines,
  verified: true
}' > "$OUT/gate-summary.json"

echo "nosql change gate passed: cassandra destructive changes detected on real repo, 5-engine matrix verified"
