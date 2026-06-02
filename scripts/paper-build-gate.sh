#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/paper-build-gate.json}"
OUT="${2:-results/generated/paper-build-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.paper-build-gate/v1" and (.claim|length) > 200 and (.gates|length) >= 1' "$SPEC" > /dev/null

for phrase in "paper build" "pdflatex" "make paper-build-gate"; do
  grep -F "$phrase" docs/paper-build.md README.md > /dev/null
done

bash scripts/paper-build.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in paper-build.json paper-tables.tex paper-build.md README.md; do
  test -s "$OUT/$output"
done

expected="$(jq -r '.gates | length' "$SPEC")"

# Every manifest entry yields a row; the generated LaTeX compiled to a non-empty PDF.
jq -e --argjson expected "$expected" '
  .version == "patchline.paper-build/v1" and
  .row_count == $expected and
  .compiled == "true"
' "$OUT/paper-build.json" > /dev/null

test -s "$OUT/paper-standalone.pdf"

jq -n --slurpfile r "$OUT/paper-build.json" '{
  version: "patchline.paper-build-gate-results/v1",
  row_count: $r[0].row_count,
  compiled: $r[0].compiled,
  verified: true
}' > "$OUT/gate-summary.json"

echo "paper-build gate passed: $(jq -r .row_count "$OUT/paper-build.json") rows generated, LaTeX compiled to PDF"
