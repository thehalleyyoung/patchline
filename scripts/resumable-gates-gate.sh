#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/resumable-gates-gate.json}"
OUT="${2:-results/generated/resumable-gates-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.resumable-gates-gate/v1" and (.claim|length) > 200 and (.repos|length) >= 3' "$SPEC" > /dev/null

for phrase in "resumable" "interrupt" "make resumable-gates-gate"; do
  grep -F "$phrase" docs/resumable-gates.md README.md > /dev/null
done

bash scripts/resumable-gates.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in resumable-gates.json resumable-gates.md README.md; do
  test -s "$OUT/$output"
done

total="$(jq '.repos | length' "$SPEC")"
interrupt_after="$(jq '.interrupt_after' "$SPEC")"
expect_resume=$((total - interrupt_after))

# The interrupted run completes exactly the prefix; the resume run recomputes none
# of the preserved work and processes only the remainder; the union is the whole
# corpus processed exactly once.
jq -e \
  --argjson total "$total" \
  --argjson interrupt_after "$interrupt_after" \
  --argjson expect_resume "$expect_resume" '
  .version == "patchline.resumable-gates/v1" and
  .total == $total and
  .completed_after_interrupt == $interrupt_after and
  .resume_processed == $expect_resume and
  .resume_skipped == $interrupt_after and
  .each_repo_processed_once == true
' "$OUT/resumable-gates.json" > /dev/null

jq -n --slurpfile r "$OUT/resumable-gates.json" '{
  version: "patchline.resumable-gates-gate-results/v1",
  total: $r[0].total,
  resume_processed: $r[0].resume_processed,
  resume_skipped: $r[0].resume_skipped,
  verified: true
}' > "$OUT/gate-summary.json"

echo "resumable-gates gate passed: interrupted prefix preserved, resume recomputes nothing, corpus processed once"
