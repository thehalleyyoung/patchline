#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/fixture-minimizer-gate.json}"
OUT="${2:-results/generated/fixture-minimizer-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.fixture-minimizer-gate/v1" and (.ecosystems|length) == 4' "$SPEC" > /dev/null

for phrase in "delta-debug" "1-minimal" "Cassandra" "oracle" "make fixture-minimizer-gate"; do
  grep -F "$phrase" docs/fixture-minimizer.md README.md > /dev/null
done

bash scripts/fixture-minimizer.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in fixture-minimizer.json fixture-minimizer.md README.md minimized/cassandra.cql; do
  test -s "$OUT/$output"
done

jq -e '
  .version == "patchline.fixture-minimizer/v1" and
  .ecosystems == 4 and
  .all_one_minimal == true and
  .all_reduced == true and
  .real_minimized_lines >= 1 and
  (.real_seed_lines > .real_minimized_lines)
' "$OUT/fixture-minimizer.json" > /dev/null

# Independently re-verify the minimized real Cassandra fixture still triggers a destructive fact.
BIN="$OUT/bin/patchline"
rm -rf "$OUT/verify"
mkdir -p "$OUT/verify/src"
cp "$OUT/minimized/cassandra.cql" "$OUT/verify/src/fixture.cql"
"$BIN" repo analyze "$OUT/verify/src" --stages inventory --no-llm --out "$OUT/verify/out" >/dev/null 2>&1
if ! grep -Eq '"kind":"nosql_change".*cassandra.*"destructive":"true"' "$OUT/verify/out/inventory/facts.jsonl"; then
  echo "minimized real fixture no longer triggers destructive fact"; exit 1
fi
rm -rf "$OUT/verify"

jq -n --slurpfile r "$OUT/fixture-minimizer.json" '{
  version: "patchline.fixture-minimizer-gate-results/v1",
  ecosystems: $r[0].ecosystems,
  real_seed_lines: $r[0].real_seed_lines,
  real_minimized_lines: $r[0].real_minimized_lines,
  verified: true
}' > "$OUT/gate-summary.json"

echo "fixture-minimizer gate passed: all ecosystems reduced and 1-minimal, real Cassandra fixture re-verified"
