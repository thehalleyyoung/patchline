#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/index-coverage-gate.json}"
OUT="${2:-results/generated/index-coverage}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.index-coverage-gate/v1" and (.indexes|length) >= 1' "$SPEC" > /dev/null

jq '
  # An index covers a query when the query needs are a subset of the index columns.
  def covers($idx; $q): ($q.needs - $idx.cols) == [];
  # Safe to drop $name iff every hot query keeps a covering index among the rest.
  def safe_to_drop($name):
    (.indexes | map(select(.name != $name))) as $rest
    | (.hot_queries) as $Q
    | ([ $Q[] as $q | select([ $rest[] | covers(.; $q) ] | any | not) | $q.name ]) as $orphaned
    | {safe: (($orphaned | length) == 0), orphaned_queries: $orphaned};
  {
    version: "patchline.index-coverage/v1",
    drop_needed: .drop_needed,
    needed_result: safe_to_drop(.drop_needed),
    drop_unused: .drop_unused,
    unused_result: safe_to_drop(.drop_unused)
  }
' "$SPEC" > "$OUT/index.json"

{
  echo "# Index-coverage analyzer"
  echo
  echo "Drop $(jq -r '.drop_needed' "$OUT/index.json") safe: $(jq -r '.needed_result.safe' "$OUT/index.json") orphaned: $(jq -rc '.needed_result.orphaned_queries' "$OUT/index.json")"
  echo "Drop $(jq -r '.drop_unused' "$OUT/index.json") safe: $(jq -r '.unused_result.safe' "$OUT/index.json")"
} > "$OUT/index.md"
cp "$OUT/index.md" "$OUT/README.md"

echo "index-coverage worker: needed_safe=$(jq -r '.needed_result.safe' "$OUT/index.json") unused_safe=$(jq -r '.unused_result.safe' "$OUT/index.json")"
