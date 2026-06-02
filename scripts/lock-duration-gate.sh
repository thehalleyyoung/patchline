#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/lock-duration-gate.json}"
OUT="${2:-results/generated/lock-duration-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.lock-duration-gate/v1" and (.claim|length) > 200 and (.operations|length) >= 1' "$SPEC" > /dev/null

for phrase in "lock-duration" "concurrent" "make lock-duration-gate"; do
  grep -F "$phrase" docs/lock-duration.md README.md > /dev/null
done

bash scripts/lock-duration.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in lock-duration.json lock-duration.md README.md; do
  test -s "$OUT/$output"
done

ms() { jq -r --arg id "$1" '.estimates[] | select(.id==$id) | .lock_ms' "$OUT/lock-duration.json"; }

# Blocking large index; concurrent collapses it to short; small table is short; defaulted
# add rewrites (blocking); nullable add on Postgres is instant. Also: concurrent < blocking,
# small < blocking (size hint load-bearing).
jq -e '
  .version == "patchline.lock-duration/v1" and
  ([.estimates[] | select(.id=="big_index")][0].class == "blocking") and
  ([.estimates[] | select(.id=="big_index_concurrent")][0].class == "short") and
  ([.estimates[] | select(.id=="small_index")][0].class == "short") and
  ([.estimates[] | select(.id=="col_default")][0].class == "blocking") and
  ([.estimates[] | select(.id=="col_nullable")][0].class == "instant") and
  ([.estimates[] | select(.id=="col_nullable")][0].lock_ms == 0) and
  (([.estimates[] | select(.id=="big_index_concurrent")][0].lock_ms) < ([.estimates[] | select(.id=="big_index")][0].lock_ms)) and
  (([.estimates[] | select(.id=="small_index")][0].lock_ms) < ([.estimates[] | select(.id=="big_index")][0].lock_ms))
' "$OUT/lock-duration.json" > /dev/null

jq -n --slurpfile r "$OUT/lock-duration.json" '{
  version: "patchline.lock-duration-gate-results/v1",
  estimates: [$r[0].estimates[] | {id, lock_ms, class}],
  verified: true
}' > "$OUT/gate-summary.json"

echo "lock-duration gate passed: blocking/short/instant classes and concurrent+size-hint effects verified"
