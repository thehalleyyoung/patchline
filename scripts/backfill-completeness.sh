#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/backfill-completeness-gate.json}"
OUT="${2:-results/generated/backfill-completeness}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.backfill-completeness-gate/v1" and (.preexisting_ids|length) >= 1' "$SPEC" > /dev/null

jq '
  def coverage($pre; $done):
    ([ $pre[] | select([ . == $done[] ] | any | not) ] | unique) as $uncovered
    | {complete: (($uncovered | length) == 0), uncovered: $uncovered};
  .preexisting_ids as $pre
  | {
      version: "patchline.backfill-completeness/v1",
      complete_case: coverage($pre; .complete_backfilled_ids),
      incomplete_case: coverage($pre; .incomplete_backfilled_ids)
    }
' "$SPEC" > "$OUT/backfill.json"

{
  echo "# Backfill-completeness checker"
  echo
  echo "Complete backfill certified: $(jq -r '.complete_case.complete' "$OUT/backfill.json")"
  echo "Incomplete backfill uncovered ids: $(jq -rc '.incomplete_case.uncovered' "$OUT/backfill.json")"
} > "$OUT/backfill.md"
cp "$OUT/backfill.md" "$OUT/README.md"

echo "backfill-completeness worker: complete=$(jq -r '.complete_case.complete' "$OUT/backfill.json") uncovered=$(jq -rc '.incomplete_case.uncovered' "$OUT/backfill.json")"
