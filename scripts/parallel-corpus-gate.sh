#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/parallel-corpus-gate.json}"
OUT="${2:-results/generated/parallel-corpus-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.parallel-corpus-gate/v1" and (.claim|length) > 200 and (.repos|length) >= 2' "$SPEC" > /dev/null

for phrase in "deterministic output ordering" "failure isolation" "make parallel-corpus-gate"; do
  grep -F "$phrase" docs/parallel-corpus.md README.md > /dev/null
done

bash scripts/parallel-corpus.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in parallel-corpus.json parallel-corpus.md README.md; do
  test -s "$OUT/$output"
done

total="$(jq '.repos | length' "$SPEC")"
expect_fail="$(jq '[.repos[] | select(.should_fail)] | length' "$SPEC")"
expect_ok=$((total - expect_fail))

# Deterministic ordering, real out-of-order completion, and failure isolation:
# the failing repo is recorded failed while every other repo still succeeds.
jq -e \
  --argjson total "$total" \
  --argjson ok "$expect_ok" \
  --argjson failed "$expect_fail" '
  .version == "patchline.parallel-corpus/v1" and
  .total == $total and
  .ok == $ok and
  .failed == $failed and
  .deterministic_order == true and
  .completion_order_differed == true and
  (.collated | length) == $total and
  (.collated == (.collated | sort_by(.name)))
' "$OUT/parallel-corpus.json" > /dev/null

jq -n --slurpfile r "$OUT/parallel-corpus.json" '{
  version: "patchline.parallel-corpus-gate-results/v1",
  total: $r[0].total,
  ok: $r[0].ok,
  failed: $r[0].failed,
  verified: true
}' > "$OUT/gate-summary.json"

echo "parallel-corpus gate passed: deterministic ordering despite out-of-order completion, failing repo isolated"
