#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/symexec-gate.json}"
OUT="${2:-results/generated/symexec-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.symexec-gate/v1" and (.claim|length) > 200 and (.variables|length) >= 1' "$SPEC" > /dev/null

for phrase in "symbolic execution" "witness" "make symexec-gate"; do
  grep -F "$phrase" docs/symexec.md README.md > /dev/null
done

bash scripts/symexec.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in symexec.json symexec.md README.md; do
  test -s "$OUT/$output"
done

# Vulnerable guard reaches an unsafe leaf with the expected witness; hardened guard
# reaches no unsafe leaf.
jq -e '
  .version == "patchline.symexec/v1" and
  .vulnerable.unsafe_reachable == true and
  .vulnerable.unsafe_witness.has_rows == true and
  .vulnerable.unsafe_witness.has_lock == false and
  (.vulnerable.reachable_leaves | index("unsafe")) != null and
  .hardened.unsafe_reachable == false and
  .hardened.unsafe_witness == null
' "$OUT/symexec.json" > /dev/null

jq -n --slurpfile r "$OUT/symexec.json" '{
  version: "patchline.symexec-gate-results/v1",
  vulnerable_unsafe: $r[0].vulnerable.unsafe_reachable,
  witness: $r[0].vulnerable.unsafe_witness,
  hardened_unsafe: $r[0].hardened.unsafe_reachable,
  verified: true
}' > "$OUT/gate-summary.json"

echo "symexec gate passed: vulnerable guard exposes unsafe path with witness, hardened guard exposes none"
