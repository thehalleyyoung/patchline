#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="${1:-results/generated/contributor-check-gate}"
rm -rf "$OUT"
mkdir -p "$OUT"

for term in \
  "patchline contributor check" \
  "roadmap-ignore" \
  "forbidden-doc-refs" \
  "focused-go-tests" \
  "make contributor-check-gate"; do
  grep -F "$term" docs/contributor-check.md > /dev/null
done

go run ./cmd/patchline contributor check \
  --root "$ROOT" \
  --out "$OUT/check" \
  --packages ./cmd/patchline \
  --gates impact-gate \
  --json > "$OUT/stdout.json"

test -s "$OUT/check/contributor-check.json"
test -s "$OUT/check/contributor-check.md"
test -s "$OUT/check/logs/roadmap-ignore.stdout"
test -s "$OUT/check/logs/focused-go-tests.stdout"
test -s "$OUT/check/logs/fast-gate-impact-gate.stdout"

jq -e '
  .version == "patchline.contributor-check/v1" and
  .mode == "run" and
  .summary.success == true and
  .summary.failed == 0 and
  .summary.passed == (.steps | length) and
  .summary.focused_test == true and
  .summary.fast_gates == 1 and
  (.hash | length) > 0 and
  ([.steps[].id] | index("roadmap-ignore")) and
  ([.steps[].id] | index("forbidden-doc-refs")) and
  ([.steps[].id] | index("gofmt")) and
  ([.steps[].id] | index("diff-check")) and
  ([.steps[].id] | index("focused-go-tests")) and
  ([.steps[].id] | index("fast-gate-impact-gate")) and
  all(.steps[]; .status == "passed" and (.output_hash | length) > 0)
' "$OUT/check/contributor-check.json" > /dev/null

grep -F "fast-gate-impact-gate" "$OUT/check/contributor-check.md" > /dev/null

jq -n \
  --slurpfile report "$OUT/check/contributor-check.json" \
  '{
    version:"patchline.contributor-check-gate-results/v1",
    root:$report[0].root,
    packages:$report[0].packages,
    gates:$report[0].gates,
    summary:$report[0].summary,
    verified:true
  }' > "$OUT/summary.json"

jq -e '.verified == true and .summary.success == true and .summary.passed >= 6' "$OUT/summary.json" > /dev/null

echo "contributor check gate passed: $(jq '.summary.passed' "$OUT/summary.json") steps"
