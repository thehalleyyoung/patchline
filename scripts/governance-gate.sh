#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/governance-gate.json}"
OUT="${2:-results/generated/governance-gate}"
mkdir -p "$(dirname "$OUT")"

jq -e '.version == "patchline.governance-gate/v1" and (.claim|length) > 200 and (.roles|length) >= 4' "$SPEC" > /dev/null

for phrase in "governance" "escalation" "make governance-gate"; do
  grep -F "$phrase" docs/governance.md README.md > /dev/null
done

bash scripts/governance.sh "$SPEC" "$OUT" > "$OUT.run.log"

for output in governance.json governance.md README.md; do
  test -s "$OUT/$output"
done

# All four mandated roles must be present; each charter must declare a scope, at
# least three responsibilities, an escalation path, and at least one accountable gate.
jq -e '
  .version == "patchline.governance/v1" and
  ([.roles[].role] | (index("maintainer") != null and index("security-reviewer") != null
     and index("research-reviewer") != null and index("ecosystem-owner") != null)) and
  (.roles | all(.[];
    (.scope|length) > 0 and
    .responsibilities >= 3 and
    (.escalation|length) > 0 and
    .accountable_gates >= 1))
' "$OUT/governance.json" > /dev/null

# Each rendered charter must contain the four mandated sections.
for role in $(jq -r '.roles[].role' "$OUT/governance.json"); do
  f="$OUT/charters/${role}.md"
  test -s "$f"
  for section in "## Scope" "## Responsibilities" "## Escalation path" "## Accountable gates"; do
    grep -Fq "$section" "$f"
  done
done

jq -n --slurpfile r "$OUT/governance.json" '{
  version: "patchline.governance-gate-results/v1",
  count: $r[0].count,
  verified: true
}' > "$OUT/gate-summary.json"

echo "governance gate passed: all four roles present, each charter complete with scope/responsibilities/escalation/gates"
