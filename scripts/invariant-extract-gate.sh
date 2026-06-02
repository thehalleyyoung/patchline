#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/invariant-extract-gate.json}"
OUT="${2:-results/generated/invariant-extract-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.invariant-extract-gate/v1" and (.claim|length) > 200 and (.schema|type=="object")' "$SPEC" > /dev/null

for phrase in "invariant extraction" "NOT NULL" "make invariant-extract-gate"; do
  grep -F "$phrase" docs/invariant-extract.md README.md > /dev/null
done

bash scripts/invariant-extract.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in invariant-extract.json invariant-extract.md README.md; do
  test -s "$OUT/$output"
done

# Four invariants extracted; safe migration preserves all; unsafe migration violates
# exactly the email NOT-NULL invariant.
jq -e '
  .version == "patchline.invariant-extract/v1" and
  .invariant_count == 4 and
  (.invariants | index("not_null:email")) != null and
  .safe.all_preserved == true and
  .unsafe.all_preserved == false and
  (.unsafe.violated == ["not_null:email"]) and
  ([.unsafe.results[] | select(.invariant=="not_null:email")][0].violating_op == "drop_not_null")
' "$OUT/invariant-extract.json" > /dev/null

jq -n --slurpfile r "$OUT/invariant-extract.json" '{
  version: "patchline.invariant-extract-gate-results/v1",
  invariant_count: $r[0].invariant_count,
  safe_preserved: $r[0].safe.all_preserved,
  unsafe_violated: $r[0].unsafe.violated,
  verified: true
}' > "$OUT/gate-summary.json"

echo "invariant-extract gate passed: safe migration preserves all invariants, unsafe violates NOT NULL"
