#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/conference-demos-gate.json}"
OUT="${2:-results/generated/conference-demos-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.conference-demos-gate/v1" and (.claim|length) > 200 and (.demos|length) >= 4' "$SPEC" > /dev/null

for phrase in "conference demo" "gate-backed" "make conference-demos-gate"; do
  grep -F "$phrase" docs/conference-demos.md README.md > /dev/null
done

bash scripts/conference-demos.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in conference-demos.json conference-demos.md README.md; do
  test -s "$OUT/$output"
done

# All four mandated audiences present; every demo step maps to a real gate target.
jq -e '
  .version == "patchline.conference-demos/v1" and
  ([.demos[].audience] | (index("Datadog") != null and index("Microsoft RISE") != null
     and index("database") != null and index("programming-languages") != null)) and
  (.demos | all(.[]; .step_count >= 1))
' "$OUT/conference-demos.json" > /dev/null

for g in $(jq -r '.demos[].steps[]' "$OUT/conference-demos.json"); do
  test -f "scripts/${g}.sh"
done

jq -n --slurpfile r "$OUT/conference-demos.json" '{
  version: "patchline.conference-demos-gate-results/v1",
  count: $r[0].count,
  verified: true
}' > "$OUT/gate-summary.json"

echo "conference-demos gate passed: 4 audiences, every demo step is a real gate target"
