#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/starter-issues-gate.json}"
OUT="${2:-results/generated/starter-issues-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.starter-issues-gate/v1" and (.claim|length) > 200 and (.cards|length) >= 1' "$SPEC" > /dev/null

for phrase in "roadmap card" "good first issue" "make starter-issues-gate"; do
  grep -F "$phrase" docs/starter-issues.md README.md > /dev/null
done

bash scripts/starter-issues.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in starter-issues.json starter-issues.md README.md; do
  test -s "$OUT/$output"
done

card_count="$(jq '.cards | length' "$SPEC")"

# Every roadmap card must produce one issue carrying all four structured fields,
# a valid gate-target name, and an artifact path under results/generated.
jq -e \
  --argjson cards "$card_count" '
  .version == "patchline.starter-issues/v1" and
  .count == $cards and
  (.issues | all(.[];
    (.title|length) > 0 and
    (.failure_mode|length) > 0 and
    (.expected_gate|test("^[a-z0-9-]+-gate$")) and
    (.artifact_path|startswith("results/generated/"))))
' "$OUT/starter-issues.json" > /dev/null

# Each rendered issue file must contain the four mandated sections.
for slug in $(jq -r '.issues[].slug' "$OUT/starter-issues.json"); do
  f="$OUT/issues/${slug}.md"
  test -s "$f"
  for section in "## Failure mode prevented" "## Expected gate" "## Artifact path" "## Acceptance criteria"; do
    grep -Fq "$section" "$f"
  done
done

jq -n --slurpfile r "$OUT/starter-issues.json" '{
  version: "patchline.starter-issues-gate-results/v1",
  count: $r[0].count,
  verified: true
}' > "$OUT/gate-summary.json"

echo "starter-issues gate passed: $(jq -r .count "$OUT/starter-issues.json") issues, each with all four structured fields and sections"
