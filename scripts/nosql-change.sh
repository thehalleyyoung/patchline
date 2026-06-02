#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/nosql-change-gate.json}"
OUT="${2:-results/generated/nosql-change}"
rm -rf "$OUT"
mkdir -p "$OUT/cache" "$OUT/analysis"

jq -e '
  .version == "patchline.nosql-change-gate/v1" and
  (.claim | length) > 200 and
  (.engines | length) == 5
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
engine="$(jq -r '.real_repo.expected_engine' "$SPEC")"
min_destructive="$(jq -r '.real_repo.minimum_destructive' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

INV="$OUT/analysis/inventory/inventory.json"
FACTS="$OUT/analysis/inventory/facts.jsonl"
test -s "$INV"

jq '[.nosql_changes[]?]' "$INV" > "$OUT/nosql-changes.json"

engine_changes="$(jq --arg e "$engine" '[.[] | select(.kind | startswith($e + ":"))] | length' "$OUT/nosql-changes.json")"
# Destructive operations are flagged in the facts stream with destructive=true.
destructive_count="$(grep -c '"destructive":"true"' "$FACTS" || true)"
distinct_engine_ops="$(jq -r --arg e "$engine" '[.[] | select(.kind | startswith($e + ":")) | .kind] | unique | length' "$OUT/nosql-changes.json")"

# The full five-engine matrix and the no-false-positive rule are covered by unit tests.
go test ./internal/project/ -run 'TestInventoryDetectsNoSQLDestructiveChanges|TestInventoryDoesNotFlagNoSQLWithoutSignal' \
  > "$OUT/unit-tests.log" 2>&1 && unit_ok=true || unit_ok=false
rm -rf internal/project/results

jq -n \
  --arg repo "$repo" \
  --arg engine "$engine" \
  --argjson engine_changes "$engine_changes" \
  --argjson destructive "${destructive_count:-0}" \
  --argjson distinct_ops "$distinct_engine_ops" \
  --argjson min "$min_destructive" \
  --argjson unit_ok "$unit_ok" \
  --slurpfile spec "$SPEC" '
  {
    version: "patchline.nosql-change/v1",
    real_repo: $repo,
    engine: $engine,
    engine_changes: $engine_changes,
    destructive_changes: $destructive,
    distinct_engine_operations: $distinct_ops,
    engines: ($spec[0].engines)
  } |
  . + {
    real_repo_destructive_detected: (.destructive_changes >= $min and .engine_changes >= 1),
    engine_matrix_verified: $unit_ok
  }
' > "$OUT/nosql-change.json"

{
  echo "# NoSQL migration and change detection"
  echo
  jq -r '"Patchline detected `" + (.engine_changes|tostring) + "` `" + .engine + "` change operations (" + (.distinct_engine_operations|tostring) + " distinct kinds), including `" + (.destructive_changes|tostring) + "` destructive operations, in the real `" + .real_repo + "` repository."' "$OUT/nosql-change.json"
  echo
  echo "## Guarantees"
  jq -r '"- real-repo destructive NoSQL changes detected: `" + (.real_repo_destructive_detected|tostring) + "`\n- five-engine matrix (MongoDB, Cassandra, Elasticsearch, Redis, DynamoDB) and no-false-positive rule verified by unit tests: `" + (.engine_matrix_verified|tostring) + "`"' "$OUT/nosql-change.json"
  echo
  echo "NoSQL stores carry just as much data-loss risk as relational databases, but their destructive operations look nothing like SQL. Patchline recognizes MongoDB collection drops and \$unset, Cassandra DROP KEYSPACE/TABLE and TRUNCATE, Elasticsearch index deletes and _delete_by_query, Redis FLUSHALL/DEL, and DynamoDB delete-table, flagging each destructive operation as searchable evidence alongside SQL migration risks."
} > "$OUT/nosql-change.md"
cp "$OUT/nosql-change.md" "$OUT/README.md"

echo "nosql change detection complete: $(jq '.engine_changes' "$OUT/nosql-change.json") cassandra changes, $(jq '.destructive_changes' "$OUT/nosql-change.json") destructive on real repo, matrix verified $(jq '.engine_matrix_verified' "$OUT/nosql-change.json")"
