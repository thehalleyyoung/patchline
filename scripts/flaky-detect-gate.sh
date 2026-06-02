#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/flaky-detect-gate.json}"
OUT="${2:-results/generated/flaky-detect-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.flaky-detect-gate/v1" and (.claim|length) > 200 and (.runs|type=="number")' "$SPEC" > /dev/null

for phrase in "flaky" "nondeterministic" "make flaky-detect-gate"; do
  grep -F "$phrase" docs/flaky-detect.md README.md > /dev/null
done

bash scripts/flaky-detect.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in flaky-detect.json flaky-detect.md README.md; do
  test -s "$OUT/$output"
done

# Deterministic candidate has exactly one distinct hash and is never flagged;
# nondeterministic candidate diverges (>1 distinct hash) and is always flagged.
jq -e '
  .version == "patchline.flaky-detect/v1" and
  .deterministic.distinct_hashes == 1 and
  .deterministic.flaky == false and
  .nondeterministic.distinct_hashes > 1 and
  .nondeterministic.flaky == true
' "$OUT/flaky-detect.json" > /dev/null

jq -n --slurpfile r "$OUT/flaky-detect.json" '{
  version: "patchline.flaky-detect-gate-results/v1",
  runs: $r[0].runs,
  deterministic_flaky: $r[0].deterministic.flaky,
  nondeterministic_flaky: $r[0].nondeterministic.flaky,
  verified: true
}' > "$OUT/gate-summary.json"

echo "flaky-detect gate passed: deterministic candidate stable, nondeterministic control flagged"
