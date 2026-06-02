#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/runtime-redaction-gate.json}"
OUT="${2:-results/generated/runtime-redaction-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.runtime-redaction-gate/v1" and (.sensitive_kinds|length)==5' "$SPEC" > /dev/null

for phrase in "Runtime redaction stability" "metric labels" "incident text" "make runtime-redaction-gate"; do
  grep -F "$phrase" docs/runtime-redaction.md README.md > /dev/null
done

bash scripts/runtime-redaction.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in runtime-evidence.redacted.jsonl runtime-redaction.json runtime-redaction.md README.md; do
  test -s "$OUT/$output"
done

minf="$(jq '.minimum_findings' "$SPEC")"

jq -e --argjson minf "$minf" '
  .version == "patchline.runtime-redaction/v1" and
  .findings >= $minf and
  .rerun_byte_identical == true and
  .raw_value_leaks == 0 and
  .deterministic_tokens == true and
  .structure_preserved == true and
  .distinct_tokens > 0
' "$OUT/runtime-redaction.json" > /dev/null

# Every token must match the documented format.
test -z "$(grep -oE '\[redacted:[a-z-]+:[0-9a-f]+\]' "$OUT/runtime-evidence.redacted.jsonl" | grep -vE '\[redacted:[a-z-]+:[0-9a-f]{12}\]' || true)"

jq -n --slurpfile r "$OUT/runtime-redaction.json" '{
  version: "patchline.runtime-redaction-gate-results/v1",
  findings: $r[0].findings,
  raw_value_leaks: $r[0].raw_value_leaks,
  rerun_byte_identical: $r[0].rerun_byte_identical,
  deterministic_tokens: $r[0].deterministic_tokens,
  verified: true
}' > "$OUT/gate-summary.json"

echo "runtime redaction gate passed: leaks $(jq '.raw_value_leaks' "$OUT/gate-summary.json"), identical $(jq '.rerun_byte_identical' "$OUT/gate-summary.json")"
