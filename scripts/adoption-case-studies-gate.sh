#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/adoption-case-studies-gate.json}"
OUT="${2:-results/generated/adoption-case-studies-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.adoption-case-studies-gate/v1" and (.claim|length) > 200 and (.case_studies|length) >= 3' "$SPEC" > /dev/null

for phrase in "case stud" "observability" "make adoption-case-studies-gate"; do
  grep -F "$phrase" docs/adoption-case-studies.md README.md > /dev/null
done

bash scripts/adoption-case-studies.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in adoption-case-studies.json adoption-case-studies.md README.md; do
  test -s "$OUT/$output"
done

# CI, observability, and migration contexts must all be represented; every cited
# capability must map to a real gate.
jq -e '
  .version == "patchline.adoption-case-studies/v1" and
  (.contexts | (index("ci") != null and index("observability") != null and index("migration") != null)) and
  (.studies | all(.[]; .capability_count >= 1))
' "$OUT/adoption-case-studies.json" > /dev/null

for g in $(jq -r '.studies[].capabilities[]' "$OUT/adoption-case-studies.json"); do
  test -f "scripts/${g}.sh"
done

jq -n --slurpfile r "$OUT/adoption-case-studies.json" '{
  version: "patchline.adoption-case-studies-gate-results/v1",
  count: $r[0].count,
  contexts: $r[0].contexts,
  verified: true
}' > "$OUT/gate-summary.json"

echo "adoption-case-studies gate passed: CI/observability/migration contexts covered, every cited capability gate-backed"
